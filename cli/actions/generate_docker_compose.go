package actions

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"

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
	if _, err := os.Stat("./docker-compose/operators/keys"); os.IsNotExist(err) {
		if err = os.Mkdir("./docker-compose/operators/keys", os.ModePerm); err != nil {
			return err
		}
	} else if err != nil {
		if err = os.Remove("./docker-compose/operators/keys"); err != nil {
			return err
		}
	}
	egnkeyCmd := fmt.Sprintf(
		"cd ./docker-compose/operators/keys && egnkey generate --key-type both --num-keys %d",
		operatorCount,
	)
	if _, _, err := runCommand(egnkeyCmd); err != nil {
		return err
	}

	// YAML config files.

	// Anvil snapshot.

	return nil
}

func runCommand(command string) (string, string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.Command("bash", "-c", command)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}
