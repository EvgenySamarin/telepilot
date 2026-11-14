/*
File: internal/tui/modal_overlay.go
Description: Top-level modal overlay dispatcher that centers the active modal dialog
within the terminal viewport.
*/

package tui

import "github.com/charmbracelet/lipgloss"

func (m *Model) renderModalOverlay() string {
	var modalBox string

	switch m.activeModal {
	case modalLoginForm:
		modalBox = m.renderLoginFormModal()
	case modalLoginQR:
		modalBox = m.renderLoginQRModal()
	case modalPrompt:
		modalBox = m.renderSingleShotModal()
	case modalNewSkill:
		modalBox = m.renderInputModal(
			m.T("input_new_skill_title"),
			m.T("input_new_skill_sub"),
			m.inputBuffer,
			m.T("input_new_skill_foot"),
		)
	case modalAIConfig:
		modalBox = m.renderAIConfigModal()
	case modalGlobalConfig:
		modalBox = m.renderGlobalConfigModal()
	case modalHelp:
		modalBox = m.renderHelpModal()
	case modalSkillPreview:
		modalBox = m.renderSkillPreviewModal()
	}

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modalBox)
}
