package actions

import (
	"errors"
	"fmt"
	"os"

	"github.com/nikolalohinski/gonja"
	"github.com/urfave/cli"
)

type Operator struct {
	Name           string
	Machine        string
	IPFSAddress    string
	LambadaAddress string
}

func GenerateDockerCompose(ctx *cli.Context) error {
	operatorCount := ctx.Uint("operators")
	if operatorCount == 0 {
		return errors.New("number of operators must be greater then 0")
	}

	// Docker compose.
	operators := make([]map[string]interface{}, operatorCount)
	for i := 1; i <= int(operatorCount); i++ {
		machine := fmt.Sprintf("machine%d", i)
		operators[i-1] = map[string]interface{}{
			"name":            fmt.Sprintf("operator%d", i),
			"machine":         machine,
			"ipfs_address":    fmt.Sprintf("%s:5001", machine),
			"lambada_address": fmt.Sprintf("%s:3033", machine),
			"config_path":     fmt.Sprintf("config-files/operators/operator%d-docker-compose.yaml", i),
		}
	}
	composeTemplate, err := gonja.FromFile("./docker-compose/docker-compose.j2")
	if err != nil {
		return err
	}
	if err != nil {
		return err
	}
	compose, err := composeTemplate.Execute(gonja.Context{"operators": operators})
	if err != nil {
		return err
	}
	if err != nil {
		return err
	}
	composeFile, err := os.Create("./docker-compose/docker-compose-gen.yaml")
	if err != nil {
		return err
	}
	defer composeFile.Close()
	if _, err = composeFile.WriteString(compose); err != nil {
		return err
	}

	// BLS and ECSDA keys.

	// YAML config files.

	// Anvil snapshot.

	return nil
}
