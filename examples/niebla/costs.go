package main

type costSource uint8

const (
	costPlacement costSource = iota + 1
	costBuilding
	costProtector
	costRobot
	costPipe
)

type CostReceipt struct {
	Source     costSource
	ID         int64
	X, Y       float64
	Oil        float64
	Lilac      float64
	Continuous bool
}

func (s *State) recordCost(
	source costSource,
	id int64,
	x, y, lilac, oil float64,
	continuous bool,
) {
	if lilac <= 0 && oil <= 0 {
		return
	}
	s.Costs = append(s.Costs, CostReceipt{
		Source: source, ID: id, X: x, Y: y,
		Lilac: lilac, Oil: oil, Continuous: continuous,
	})
}
