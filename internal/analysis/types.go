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
}
