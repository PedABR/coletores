package main

import (
	"fmt"
	"io"
	"math"
	"os"

	storage "github.com/dadosjusbr/storage"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

const tableXPath = `//*[@id="tblDetalhamentoFolhaPagamentoPessoal"]`

const (
	// Basic info
	nameXPath      = `//*[@class="c01"]`
	roleXPath      = `//*[@class="c02"]`
	workplaceXPath = `//*[@class="c03 capitalize"]`
	// Income
	wageXPath  = `//*[@class="c04"]`
	perksXPath = `//*[@class="c06"]`
	// Income.Others
	personalBenefitsXPath = `//*[@class="c05"]`
	positionOfTrustXPath  = `//*[@class="c17"]`
	eventualBenefitsXPath = `//*[@class="c07"]`
	gratificationXPath    = `//*[@class="c08"]`
	// Discount
	prevContributionXPath = `//*[@class="c10"]`
	ceilRetentionXPath    = `//*[@class="c13"]`
	incomeTaxXPath        = `//*[@class="c11"]`
	sundryXPath           = `//*[@class="c12"]`
	// Origin Position
	originPositionXPath = `//*[@class="c16"]`
)

// parser takes a filepath and will return a slice of employees extracted from the file
func parse(filePath string) ([]storage.Employee, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening file (%s): %q", filePath, err)
	}

	table, err := loadTable(f)
	if err != nil {
		return nil, fmt.Errorf("error while loading data table from %s: %q", filePath, err)
	}

	e, err := employeeRecords(table)
	if err != nil {
		return nil, fmt.Errorf("error while parsing data from table (%s): %q", filePath, err)
	}

	return e, nil
}

// loadTable will load a the correct data table from an io.Reader that should hold an html page.
func loadTable(r io.Reader) (*html.Node, error) {
	doc, err := htmlquery.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("error parsing file html tree: %q", err)
	}
	table, err := htmlquery.Query(doc, tableXPath)
	if err != nil {
		return nil, fmt.Errorf("error making xpathquery(%s) in file: %q", tableXPath, err)
	}
	if table == nil {
		return nil, fmt.Errorf("error finding data table in file: %q", err)
	}

	return table, nil
}

// employeeRecords will retrieve a list of employees from the data table
func employeeRecords(table *html.Node) ([]storage.Employee, error) {
	records, err := htmlquery.QueryAll(table, "//tr")
	if err != nil {
		return nil, fmt.Errorf("error while querying data table for tr elements: %q", err)
	}

	var employees []storage.Employee

	for i, row := range records[1:] {
		e, err := newEmployee(row)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error trying to parse employee columns(position %d): %q. Row: %v", i, err, row)
		}
		employees = append(employees, e)
	}
	return employees, nil
}

// newEmployee will create a new employee from a row.
func newEmployee(row *html.Node) (storage.Employee, error) {
	// Creating employee struct as needed.
	e := storage.Employee{
		Income:    &storage.IncomeDetails{Other: &storage.Funds{}, Perks: &storage.Perks{}},
		Discounts: &storage.Discount{},
	}

	if err := employeeBasicInfo(row, &e); err != nil {
		return e, fmt.Errorf("error retrieving employee basic info: %q", err)
	}
	if err := employeeIncome(row, e.Income); err != nil {
		return e, fmt.Errorf("error retrieving employee income info: %q", err)
	}
	if err := employeeDiscounts(row, e.Discounts); err != nil {
		return e, fmt.Errorf("error retrieving employee discounts info: %q", err)
	}
	return e, nil
}

// employeeBasicInfo will fetch basic info from the rows.
func employeeBasicInfo(row *html.Node, e *storage.Employee) error {
	if err := retrieveString(row, &e.Name, nameXPath); err != nil {
		return fmt.Errorf("error retrieving name: %q", err)
	}
	if err := retrieveString(row, &e.Role, roleXPath); err != nil {
		return fmt.Errorf("error retrieving role: %q", err)
	}
	if err := retrieveString(row, &e.Workplace, workplaceXPath); err != nil {
		return fmt.Errorf("error retrieving workplace: %q", err)
	}
	e.Active = active(e.Role)
	return nil
}

// employeeIncome will fetch Income info from the rows.
func employeeIncome(row *html.Node, i *storage.IncomeDetails) error {
	if err := retrieveFloat(row, &i.Wage, wageXPath); err != nil {
		return fmt.Errorf("error retrieving Wage: %q", err)
	}
	if err := retrieveFloat(row, &i.Perks.Total, perksXPath); err != nil {
		return fmt.Errorf("error retrieving perks: %q", err)
	}
	if err := employeeIncomeOthers(row, i.Other); err != nil {
		return fmt.Errorf("error retrieving other incomes: %q", err)
	}
	i.Total = totalIncome(*i)
	return nil
}

// employeeIncomeOthers will fetch other incomes info from the rows.
func employeeIncomeOthers(row *html.Node, o *storage.Funds) error {
	if err := retrieveFloat(row, &o.PersonalBenefits, personalBenefitsXPath); err != nil {
		return fmt.Errorf("error retrieving personal benefits: %q", err)
	}
	if err := retrieveFloat(row, &o.EventualBenefits, eventualBenefitsXPath); err != nil {
		return fmt.Errorf("error retrieving eventual benefits: %q", err)
	}
	if err := retrieveFloat(row, &o.PositionOfTrust, positionOfTrustXPath); err != nil {
		return fmt.Errorf("error retrieving position of trust: %q", err)
	}
	if err := retrieveFloat(row, &o.Gratification, gratificationXPath); err != nil {
		return fmt.Errorf("error retrieving gratification: %q", err)
	}
	if err := retrieveFloat(row, &o.OriginPosition, originPositionXPath); err != nil {
		return fmt.Errorf("error retrieving origin position: %q", err)
	}
	o.Total = totalFunds(*o)
	return nil
}

// employeeDiscount will fetch discounts info from the row.
func employeeDiscounts(row *html.Node, d *storage.Discount) error {
	if err := retrieveFloat(row, &d.PrevContribution, prevContributionXPath); err != nil {
		return fmt.Errorf("error retrieving PrevContribution: %q", err)
	}
	if err := retrieveFloat(row, &d.CeilRetention, ceilRetentionXPath); err != nil {
		return fmt.Errorf("error retrieving ceilRetention: %q", err)
	}
	if err := retrieveFloat(row, &d.IncomeTax, incomeTaxXPath); err != nil {
		return fmt.Errorf("error retrieving incomeTax: %q", err)
	}
	var sundryV float64
	if err := retrieveFloat(row, &sundryV, sundryXPath); err != nil {
		return fmt.Errorf("error retrieving incomeTax: %q", err)
	}
	d.Others = make(map[string]float64)
	d.Others["other_discounts"] = sundryV
	d.Total = totalDiscounts(*d)
	return nil
}

// active returns a boolean representing if the employee is active or not.
func active(role string) bool {
	return role != "Inativo"
}

// totalDiscounts returns the sum of discounts.
func totalDiscounts(d storage.Discount) float64 {
	total := getFloat64Value(d.PrevContribution) + getFloat64Value(d.CeilRetention) + getFloat64Value(d.IncomeTax) + sumMapValues(d.Others)
	return math.Round(total*100) / 100
}

// totalFunds returns the sum of funds.
func totalFunds(f storage.Funds) float64 {
	total := getFloat64Value(f.PersonalBenefits) + getFloat64Value(f.EventualBenefits) +
		getFloat64Value(f.PositionOfTrust) + getFloat64Value(f.Daily) + getFloat64Value(f.Gratification) + getFloat64Value(f.OriginPosition) + sumMapValues(f.Others)
	return math.Round(total*100) / 100
}

// grossIncome returns the sum of incomes.
func totalIncome(in storage.IncomeDetails) float64 {
	total := getFloat64Value(in.Wage) + in.Perks.Total + in.Other.Total
	return math.Round(total*100) / 100
}
