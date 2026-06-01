// Package skill 提供可复用的声明式工作流模板。
// Skill 由 YAML 定义，AI 通过 SkillTool 按名称调用执行。
// 对应 claude-code 的 Skill 系统概念。
package skill

// Manifest 是单个 Skill 的完整定义（对应一个 YAML 文件）
type Manifest struct {
	Name        string     `yaml:"name" json:"name"`
	Description string     `yaml:"description" json:"description"`
	Category    string     `yaml:"category" json:"category"`  // ops | file | dns | system
	RiskLevel   string     `yaml:"risk" json:"risk"`          // readonly | mutation | dangerous
	Steps       []StepDef  `yaml:"steps" json:"steps"`
	Rollback    []StepDef  `yaml:"rollback,omitempty" json:"rollback,omitempty"`
}

// StepDef 单个步骤定义
type StepDef struct {
	ID      int            `yaml:"id" json:"id"`
	Tool    string         `yaml:"tool" json:"tool"`
	Args    map[string]any `yaml:"args" json:"args"`
	Depends []int          `yaml:"depends,omitempty" json:"depends,omitempty"`
	Purpose string         `yaml:"purpose" json:"purpose"`
}

// StepStatus 步骤执行状态
type StepStatus string

const (
	StepPending    StepStatus = "pending"
	StepRunning    StepStatus = "running"
	StepDone       StepStatus = "done"
	StepFailed     StepStatus = "failed"
	StepSkipped    StepStatus = "skipped"
)

// Execution 表示一次 Skill 执行
type Execution struct {
	SkillName   string       `json:"skill_name"`
	Steps       []StepResult `json:"steps"`
	CurrentStep int          `json:"current_step"`
	TotalSteps  int          `json:"total_steps"`
	Status      StepStatus   `json:"status"`
}

// StepResult 单步执行结果
type StepResult struct {
	ID       int        `json:"id"`
	Tool     string     `json:"tool"`
	Purpose  string     `json:"purpose"`
	Status   StepStatus `json:"status"`
	Result   string     `json:"result,omitempty"`
	Error    string     `json:"error,omitempty"`
}
