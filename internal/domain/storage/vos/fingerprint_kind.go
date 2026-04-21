package vos

type FingerprintKind int

const (
	KindCode        FingerprintKind = iota
	KindInstruction FingerprintKind = iota
	KindVars        FingerprintKind = iota
)

const (
	KindCodeString        = "code"
	KindInstructionString = "instruction"
	KindVarsString        = "vars"
)

func (k FingerprintKind) String() string {
	switch k {
	case KindCode:
		return KindCodeString
	case KindInstruction:
		return KindInstructionString
	case KindVars:
		return KindVarsString
	default:
		return "unknown"
	}
}
