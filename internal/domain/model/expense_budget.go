package model

import "time"

type ExpenseBudget struct {
	Exercise              int       `db:"exercise"`
	SuperiorOrganCode     int64     `db:"superior_organ_code"`
	SuperiorOrganName     string    `db:"superior_organ_name"`
	SubordinateAgencyCode int64     `db:"subordinate_agency_code"`
	SubordinateAgencyName string    `db:"subordinate_agency_name"`
	BudgetaryUnitCode     int64     `db:"budgetary_unit_code"`
	BudgetaryUnitName     string    `db:"budgetary_unit_name"`
	FunctionCode          int64     `db:"function_code"`
	FunctionName          string    `db:"function_name"`
	SubfunctionCode       int64     `db:"subfunction_code"`
	SubfunctionName       string    `db:"subfunction_name"`
	BudgetProgramCode     string    `db:"budget_program_code"`
	BudgetProgramName     string    `db:"budget_program_name"`
	ActionCode            string    `db:"action_code"`
	ActionName            string    `db:"action_name"`
	EconomicCategoryCode  int64     `db:"economic_category_code"`
	EconomicCategory      string    `db:"economic_category"`
	ExpenseGroupCode      int16     `db:"expense_group_code"`
	ExpenseGroupName      string    `db:"expense_group_name"`
	ExpenseElementCode    int64     `db:"expense_element_code"`
	ExpenseElementName    string    `db:"expense_element_name"`
	InitialBudget         float64   `db:"initial_budget"`
	UpdatedBudget         float64   `db:"updated_budget"`
	CommittedBudget       float64   `db:"committed_budget"`
	ExecutedBudget        float64   `db:"executed_budget"`
	PercentExecutedBudget float64   `db:"percent_executed_budget"`
	InsertedAt            time.Time `db:"inserted_at"`
	UpdatedAt             time.Time `db:"updated_at"`
}
