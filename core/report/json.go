package report

import (
	"encoding/json"
	"fmt"
)

func JsonFormat(findings []Finding) {
	j, err := json.Marshal(findings)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(j))
}
