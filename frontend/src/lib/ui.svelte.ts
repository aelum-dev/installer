// Transient, view-only wizard state.
//
// Dialog visibility, inline popup errors, and the not-yet-committed manual
// partition preview live here instead of in installState, so the durable
// install data (what gets saved and synced to the backend) never mixes with
// ephemeral UI flags.
export const uiState = $state({
	// LUKS passphrase popup and its inline validation error.
	luksDialogOpen: false,
	luksError: '',

	// Destructive-install confirmation dialog (Confirmation page).
	showConfirmDialog: false,

	// Pending "add partition" size in MiB, previewed by the manual-mode top bar
	// before the partition is actually added to the plan.
	pendingAddMB: 0,
});
