package stubs

// InstructionType represents the type of Dockerfile instruction
type InstructionType string

const (
	InstructionFROM    InstructionType = "FROM"
	InstructionCOPY    InstructionType = "COPY"
	InstructionRUN     InstructionType = "RUN"
	InstructionWORKDIR InstructionType = "WORKDIR"
	InstructionENV     InstructionType = "ENV"
	InstructionCMD     InstructionType = "CMD"
)

// Instruction represents a Dockerfile instruction
type Instruction interface {
	Type() InstructionType // Returns the instruction type
	Text() string          // Returns raw instruction text
}

// FromInstruction represents a FROM instruction
type FromInstruction struct {
	Image string
	Tag   string
	Raw   string
}

func (f *FromInstruction) Type() InstructionType { return InstructionFROM }
func (f *FromInstruction) Text() string          { return f.Raw }

// CopyInstruction represents a COPY instruction
type CopyInstruction struct {
	Sources []string // Source paths
	Dest    string   // Destination path
	Raw     string
}

func (c *CopyInstruction) Type() InstructionType { return InstructionCOPY }
func (c *CopyInstruction) Text() string          { return c.Raw }

// RunInstruction represents a RUN instruction
type RunInstruction struct {
	Command string
	Raw     string
}

func (r *RunInstruction) Type() InstructionType { return InstructionRUN }
func (r *RunInstruction) Text() string          { return r.Raw }

// WorkdirInstruction represents a WORKDIR instruction
type WorkdirInstruction struct {
	Path string
	Raw  string
}

func (w *WorkdirInstruction) Type() InstructionType { return InstructionWORKDIR }
func (w *WorkdirInstruction) Text() string          { return w.Raw }

// EnvInstruction represents an ENV instruction
type EnvInstruction struct {
	Key   string
	Value string
	Raw   string
}

func (e *EnvInstruction) Type() InstructionType { return InstructionENV }
func (e *EnvInstruction) Text() string          { return e.Raw }

// CmdInstruction represents a CMD instruction
type CmdInstruction struct {
	Command []string
	Raw     string
}

func (c *CmdInstruction) Type() InstructionType { return InstructionCMD }
func (c *CmdInstruction) Text() string          { return c.Raw }
