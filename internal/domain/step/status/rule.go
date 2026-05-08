package status

const (
	PipelineUrlParam = "pipeline_url"
	ProjectUrlParam  = "project_url"
	EnvironmentParam = "environment"
	StepParam        = "step"
)

type Rule interface {
	Name() string
	Evaluate(ctx RuleContext) (Decision, error)
}
