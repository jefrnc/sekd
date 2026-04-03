package report

import (
	"encoding/json"
	"fmt"

	"github.com/jefrnc/sekd/internal/analysis"
)

func RenderJSON(r *analysis.Report) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
