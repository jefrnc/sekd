package edgar

import "time"

type CompanyTicker struct {
	CIK    int    `json:"cik_str"`
	Ticker string `json:"ticker"`
	Title  string `json:"title"`
}

type Submissions struct {
	CIK         string `json:"cik"`
	EntityType  string `json:"entityType"`
	Name        string `json:"name"`
	SIC         string `json:"sic"`
	SICDesc     string `json:"sicDescription"`
	StateOfInc  string `json:"stateOfIncorporation"`
	FiscalYrEnd string `json:"fiscalYearEnd"`
	Filings     struct {
		Recent FilingRecent `json:"recent"`
		Files  []struct {
			Name string `json:"name"`
		} `json:"files"`
	} `json:"filings"`
}

type FilingRecent struct {
	AccessionNumber []string `json:"accessionNumber"`
	FilingDate      []string `json:"filingDate"`
	Form            []string `json:"form"`
	PrimaryDocument []string `json:"primaryDocument"`
}

type Filing struct {
	AccessionNumber string
	FilingDate      time.Time
	Form            string
	PrimaryDocument string
}

type CompanyFacts struct {
	CIK        int    `json:"cik"`
	EntityName string `json:"entityName"`
	Facts      struct {
		USGAAP map[string]FactEntry `json:"us-gaap"`
	} `json:"facts"`
}

type FactEntry struct {
	Label string `json:"label"`
	Units map[string][]FactDatapoint `json:"units"`
}

type FactDatapoint struct {
	Val   float64 `json:"val"`
	End   string  `json:"end"`
	FY    int     `json:"fy"`
	FP    string  `json:"fp"`
	Form  string  `json:"form"`
	Filed string  `json:"filed"`
}

type SharesDatapoint struct {
	Shares float64
	Date   time.Time
	Form   string
	Filed  time.Time
}
