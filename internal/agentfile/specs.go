package agentfile

import "path/filepath"

// GlobalSpecs returns the specifications for all agent instruction files installed at user level (~/).
func GlobalSpecs() []Spec {
	return []Spec{
		{RelPath: filepath.Join(".claude", "skills", "rimba", "SKILL.md"), Kind: KindWhole, Content: globalClaudeSkillContent},
		{RelPath: filepath.Join(".cursor", "rules", "rimba.mdc"), Kind: KindWhole, Content: globalCursorContent},
		{RelPath: filepath.Join(".github", "copilot-instructions.md"), Kind: KindBlock, Content: globalCopilotBlock},
		{RelPath: filepath.Join(".codex", "AGENTS.md"), Kind: KindBlock, Content: globalCodexBlock},
		{RelPath: filepath.Join(".gemini", "GEMINI.md"), Kind: KindBlock, Content: globalGeminiBlock},
		{RelPath: filepath.Join(".codeium", "windsurf", "memories", "global_rules.md"), Kind: KindBlock, Content: globalWindsurfBlock},
		{RelPath: filepath.Join(".roo", "rules", "rimba.md"), Kind: KindWhole, Content: globalRooContent},
	}
}

// ProjectSpecs returns the specifications for all agent instruction files installed at project level.
func ProjectSpecs() []Spec {
	return []Spec{
		{RelPath: filepath.Join(".claude", "skills", "rimba", "SKILL.md"), Kind: KindWhole, Content: claudeSkillContent},
		{RelPath: filepath.Join(".cursor", "rules", "rimba.mdc"), Kind: KindWhole, Content: cursorContent},
		{RelPath: filepath.Join(".github", "copilot-instructions.md"), Kind: KindBlock, Content: copilotBlock},
		{RelPath: "AGENTS.md", Kind: KindBlock, Content: agentsBlock},
		{RelPath: "GEMINI.md", Kind: KindBlock, Content: geminiBlock},
		{RelPath: filepath.Join(".windsurf", "rules", "rimba.md"), Kind: KindWhole, Content: windsurfContent},
		{RelPath: filepath.Join(".clinerules", "rimba.md"), Kind: KindWhole, Content: rooContent},
	}
}
