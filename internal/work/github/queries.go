package github

// The GraphQL documents, and the shapes they answer with, in one place: the
// two halves of the same contract, and the thing to read against GitHub's
// schema when something stops working.
//
// Every operation carries a name. GitHub logs it, and a nameless document in
// an audit log is a nameless document in an incident.
//
// The owner of a board is asked for twice, as an organization and as a user,
// in one document. Which one it is is not something a configuration should
// have to state: exactly one of the two answers and the other is null, and a
// board that moved from a person to an org keeps working.

const viewerQuery = `query Viewer {
  viewer { login }
}`

const projectQuery = `query Project($owner: String!, $project: Int!) {
  organization(login: $owner) { projectV2(number: $project) { id title ...Fields } }
  user(login: $owner) { projectV2(number: $project) { id title ...Fields } }
}

fragment Fields on ProjectV2 {
  fields(first: 50) {
    nodes {
      __typename
      ... on ProjectV2SingleSelectField { id name options { id name color description } }
    }
  }
}`

// itemsQuery is the queue. It asks for a page of the board in board order and
// nothing else: no bodies, no comments, no field values beyond the two columns
// the fleet reads. Points are charged per hundred objects returned, so a
// comment stream per item would turn a five-second poll from about a point
// into about a hundred.
//
// The filter is the board's own search syntax, so the server does the
// narrowing: one repository's issues, and nothing that is a pull request.
const itemsQuery = `query Items($owner: String!, $project: Int!, $filter: String!, $agentField: String!, $statusField: String!) {
  organization(login: $owner) { projectV2(number: $project) { ...Page } }
  user(login: $owner) { projectV2(number: $project) { ...Page } }
}

fragment Page on ProjectV2 {
  items(first: 100, query: $filter) {
    nodes {
      content { __typename ... on Issue { number title state createdAt } }
      agent: fieldValueByName(name: $agentField) { ... on ProjectV2ItemFieldSingleSelectValue { name } }
      status: fieldValueByName(name: $statusField) { ... on ProjectV2ItemFieldSingleSelectValue { name } }
    }
  }
}`

// issueQuery is one task in full. This is the call a person makes — the fleet
// reads the body, the checklist and the whole thread, and pays for them.
//
// projectItems cannot be filtered by project, so a page of them is read and
// the one whose project is ours is picked out. An issue on several boards has
// several items; only one of them is this queue.
const issueQuery = `query Issue($owner: String!, $project: Int!, $repoOwner: String!, $repoName: String!, $issue: Int!, $agentField: String!, $statusField: String!) {
  organization(login: $owner) { projectV2(number: $project) { id } }
  user(login: $owner) { projectV2(number: $project) { id } }
  repository(owner: $repoOwner, name: $repoName) {
    issue(number: $issue) {
      number title body state createdAt
      comments(last: 100) { nodes { body author { login } } }
      projectItems(first: 10) {
        nodes {
          id
          project { id }
          agent: fieldValueByName(name: $agentField) { ... on ProjectV2ItemFieldSingleSelectValue { name } }
          status: fieldValueByName(name: $statusField) { ... on ProjectV2ItemFieldSingleSelectValue { name } }
        }
      }
    }
  }
}`

// targetQuery is everything a write needs about one task: its node id, its
// item on our board, and the board's fields. One document for all four writes,
// so "is this the fleet's work" is answered in exactly one place.
//
// It is also what makes the answer honest: an issue that is in the repository
// but not an item of the configured project is somebody else's list, and the
// fleet says so rather than adding it to a board it was not told to touch.
const targetQuery = `query Target($owner: String!, $project: Int!, $repoOwner: String!, $repoName: String!, $issue: Int!) {
  organization(login: $owner) { projectV2(number: $project) { id ...Fields } }
  user(login: $owner) { projectV2(number: $project) { id ...Fields } }
  repository(owner: $repoOwner, name: $repoName) {
    issue(number: $issue) {
      id
      projectItems(first: 10) { nodes { id project { id } } }
    }
  }
}`

// createQuery is what Create needs before it writes anything: the repository
// to write into, the board to put the issue on, and the board's fields — so
// that a routing key that is not an option is refused before an issue exists
// rather than after.
const createQuery = `query Create($owner: String!, $project: Int!, $repoOwner: String!, $repoName: String!) {
  organization(login: $owner) { projectV2(number: $project) { id ...Fields } }
  user(login: $owner) { projectV2(number: $project) { id ...Fields } }
  repository(owner: $repoOwner, name: $repoName) { id }
}`

// createIssueMutation creates the issue and puts it on the board in one
// mutation. Doing it in two — create, then add — is what leaves an orphan when
// the second call fails: an issue nobody asked for, in a repository, routed to
// no agent and on no board.
const createIssueMutation = `mutation CreateIssue($repo: ID!, $project: ID!, $title: String!, $body: String!) {
  createIssue(input: {repositoryId: $repo, projectV2Ids: [$project], title: $title, body: $body}) {
    issue {
      id
      number
      projectItems(first: 10) { nodes { id project { id } } }
    }
  }
}`

const setFieldMutation = `mutation SetField($project: ID!, $item: ID!, $field: ID!, $option: String!) {
  updateProjectV2ItemFieldValue(input: {projectId: $project, itemId: $item, fieldId: $field, value: {singleSelectOptionId: $option}}) {
    projectV2Item { id }
  }
}`

const addCommentMutation = `mutation AddComment($subject: ID!, $body: String!) {
  addComment(input: {subjectId: $subject, body: $body}) { commentEdge { node { id } } }
}`

// closeMutation closes the issue and writes the verdict in one document. The
// mutations in a document run in order, and closing is first: a failure to
// comment then leaves an issue the fleet has finished with, whose outcome is
// honestly unknown, rather than one that is still open and about to be run a
// second time.
const closeMutation = `mutation Close($issue: ID!, $reason: IssueClosedStateReason!, $body: String!) {
  closeIssue(input: {issueId: $issue, stateReason: $reason}) { issue { id state stateReason } }
  addComment(input: {subjectId: $issue, body: $body}) { commentEdge { node { id } } }
}`

// The two schema mutations behind `auth github`. A single-select field's
// options can only be written as a whole set, which is why adding one means
// sending every option back with it — including the ones that are already
// there, with their ids, or GitHub clears the field values that point at them.
const createFieldMutation = `mutation CreateField($project: ID!, $name: String!, $options: [ProjectV2SingleSelectFieldOptionInput!]!) {
  createProjectV2Field(input: {projectId: $project, name: $name, dataType: SINGLE_SELECT, singleSelectOptions: $options}) {
    projectV2Field { ... on ProjectV2SingleSelectField { id name options { id name color description } } }
  }
}`

const updateFieldMutation = `mutation UpdateField($field: ID!, $options: [ProjectV2SingleSelectFieldOptionInput!]!) {
  updateProjectV2Field(input: {fieldId: $field, singleSelectOptions: $options}) {
    projectV2Field { ... on ProjectV2SingleSelectField { id name options { id name color description } } }
  }
}`

// The shapes those documents answer with. One type per thing the fleet cares
// about, shared between documents: a document that does not ask for a part
// leaves it zero, so one type describes the board whichever question was
// asked.

// boardHalf is one of the two aliased answers, organization(login:) or
// user(login:). Exactly one of them carries the project.
type boardHalf struct {
	ProjectV2 *project `json:"projectV2"`
}

// project is a board: its identity, its fields, and — when the document asked
// for items — the page of them that is the queue.
type project struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Fields struct {
		Nodes []field `json:"nodes"`
	} `json:"fields"`
	Items struct {
		Nodes []listItem `json:"nodes"`
	} `json:"items"`
}

// field is a project field as far as the fleet is concerned. Only a
// single-select field can carry a routing key or a column, so the others
// arrive as a bare typename and nothing here pretends otherwise.
type field struct {
	Typename string   `json:"__typename"`
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Options  []option `json:"options"`
}

func (f *field) singleSelect() bool { return f.Typename == "ProjectV2SingleSelectField" }

// field is the board's field of that name, if it has one.
func (p *project) field(name string) *field {
	for i := range p.Fields.Nodes {
		if p.Fields.Nodes[i].Name == name {
			return &p.Fields.Nodes[i]
		}
	}
	return nil
}

func (f *field) option(name string) *option {
	for i := range f.Options {
		if f.Options[i].Name == name {
			return &f.Options[i]
		}
	}
	return nil
}

// option is one option of a single-select field. The id is what a write
// carries; the name is what a person chose.
type option struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
}

// listItem is one item of the queue's page: what the issue says about itself,
// and the two columns the fleet reads.
type listItem struct {
	Content *issueNode `json:"content"`
	Agent   fieldValue `json:"agent"`
	Status  fieldValue `json:"status"`
}

// issueNode is an item's content when that content is an issue. A draft card
// and a pull request decode to nothing here, which is how they are told apart
// from work: they are on the board, and they are not the fleet's.
type issueNode struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	State     string `json:"state"`
	CreatedAt string `json:"createdAt"`
}

// comment is one entry of an issue's thread.
type comment struct {
	Body   string `json:"body"`
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
}

// fieldValue is a single-select value as it hangs off an item. Unset values
// come back null and read as the empty string, which is a board that has not
// been told, not a board that said something.
type fieldValue struct {
	Name string `json:"name"`
}

// itemNode is an issue's membership of a board: the item, and which project it
// belongs to.
type itemNode struct {
	ID      string `json:"id"`
	Project struct {
		ID string `json:"id"`
	} `json:"project"`
	Agent  fieldValue `json:"agent"`
	Status fieldValue `json:"status"`
}
