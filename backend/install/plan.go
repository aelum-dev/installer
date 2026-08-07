package install

import "aestheticos-installer/backend/shared"

// Plan defines an ordered list of installation steps with relative weights
// for progress bar calculation.
type Plan struct {
	Steps   []Step
	Weights []int
}

// DefaultPlan returns the canonical installation plan.
func DefaultPlan() *Plan {
	return &Plan{
		Steps: DefaultSteps(),
		Weights: []int{
			shared.WeightFormatDisk,
			shared.WeightMountFilesystem,
			shared.WeightInstallBase,
			shared.WeightInstallDesktop,
			shared.WeightSetupUsers,
			shared.WeightConfigureSystem,
		},
	}
}

// cumulativePercentages calculates the cumulative percentage at each step boundary
// based on plan weights. Returns a slice of length len(steps)+1, where result[0] = 0
// and result[len] = 100.
func (p *Plan) cumulativePercentages() []int {
	totalWeight := 0
	for _, w := range p.Weights {
		totalWeight += w
	}

	result := make([]int, len(p.Steps)+1)
	result[0] = 0
	cumulativeWeight := 0
	for i := range p.Steps {
		if i < len(p.Weights) {
			cumulativeWeight += p.Weights[i]
			result[i+1] = int(float64(cumulativeWeight) / float64(totalWeight) * 100)
		}
	}
	return result
}

// stepWeights returns the canonical weight constants for installation steps.
func stepWeights() []int {
	return []int{
		shared.WeightFormatDisk,
		shared.WeightMountFilesystem,
		shared.WeightInstallBase,
		shared.WeightInstallDesktop,
		shared.WeightSetupUsers,
		shared.WeightConfigureSystem,
	}
}
