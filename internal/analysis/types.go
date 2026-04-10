package analysis

type RiskFlag string

const (
	FlagHighDilution    RiskFlag = "HIGH_DILUTION"
	FlagRecentATM       RiskFlag = "RECENT_ATM"
	FlagInsiderSelling  RiskFlag = "INSIDER_SELLING"
	FlagLowFloat        RiskFlag = "LOW_FLOAT"
	FlagHighShortInt    RiskFlag = "HIGH_SHORT_INTEREST"
	FlagShelfReg        RiskFlag = "SHELF_REGISTRATION"
	FlagMassiveAuthorized RiskFlag = "MASSIVE_AUTHORIZED_SHARES"
	FlagWarrantsITM     RiskFlag = "WARRANTS_IN_THE_MONEY"
	FlagShelfCapacity   RiskFlag = "LARGE_SHELF_REMAINING"
)

type RiskFlagDetail struct {
	Flag        RiskFlag
	Label       string
	Description string
	Severity    string // HIGH, MEDIUM, LOW
	Points      int    // deduction from score
}

type DilutionAnalysis struct {
	SharesHistory       []SharesEntry
	DilutionRate6M      float64
	DilutionRate12M     float64
	ATMFilings          []FilingSummary
	ShelfRegistrations  []FilingSummary
	AuthorizedShares    float64
	OutstandingShares   float64
	AuthorizedRatio     float64
}

type SharesEntry struct {
	Date   string
	Shares float64
	Form   string
}

type FilingSummary struct {
	Date string
	Form string
}

type InsiderSummary struct {
	Form4Count   int
	Period       string
}

type DDScore struct {
	Score   int
	Grade   string
	Summary string
}

type Report struct {
	Ticker          string
	CompanyName     string
	CIK             string
	Sector          string
	Industry        string
	Country         string
	MarketCap       string
	Price           string
	Float           string
	ShortFloat      string
	InsiderOwn      string
	InstOwn         string
	Volume          string
	AvgVolume       string
	RelVolume       string
	Dilution        DilutionAnalysis
	Insider         InsiderSummary
	RiskFlags       []RiskFlagDetail
	Score           DDScore
	Deep            *DeepDilution `json:",omitempty"`

	// LatestAccession is the accession number of the most recent filing of
	// any form type. Populated by the builder so callers (e.g. watchlist
	// scan) can detect "anything new since last time I looked" without
	// re-fetching submissions.
	LatestAccession  string `json:",omitempty"`
	LatestFilingDate string `json:",omitempty"`
	LatestFilingForm string `json:",omitempty"`
}
