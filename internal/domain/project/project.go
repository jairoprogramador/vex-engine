package project

type Project struct {
	id   ProjectID
	name ProjectName
	team ProjectTeam
	org  ProjectOrg
	url  ProjectURL
	ref  ProjectRef
}

func NewProject(id ProjectID, name ProjectName, team ProjectTeam, org ProjectOrg, url ProjectURL, ref ProjectRef) (*Project, error) {
	return &Project{id: id, name: name, team: team, org: org, url: url, ref: ref}, nil
}

func (p *Project) ID() ProjectID     { return p.id }
func (p *Project) Name() ProjectName { return p.name }
func (p *Project) Team() ProjectTeam { return p.team }
func (p *Project) Org() ProjectOrg   { return p.org }
func (p *Project) URL() ProjectURL   { return p.url }
func (p *Project) Ref() ProjectRef   { return p.ref }
