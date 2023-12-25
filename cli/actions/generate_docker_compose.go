package actions

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/nikolalohinski/gonja"
	"github.com/urfave/cli"
)

func GenerateDockerCompose(ctx *cli.Context) error {

	operatorCount := ctx.Uint("operators")
	if operatorCount == 0 {
		return errors.New("number of operators must be greater then 0")
	}

	// Docker compose.
	operators := make([]map[string]interface{}, operatorCount)
	configPaths := make([]string, operatorCount)
	for i := 1; i <= int(operatorCount); i++ {
		machine := fmt.Sprintf("machine%d", i)
		configPaths[i-1] = filepath.Join(
			"./docker-compose/operators/configs",
			fmt.Sprintf("operator%d.yaml", i),
		)
		operators[i-1] = map[string]interface{}{
			"name":            fmt.Sprintf("operator%d", i),
			"machine":         machine,
			"ipfs_address":    fmt.Sprintf("%s:5001", machine),
			"lambada_address": fmt.Sprintf("%s:3033", machine),
			"config_path":     configPaths[i-1],
		}
	}
	cofingTmpl, err := gonja.FromFile("./docker-compose/docker-compose.j2")
	if err != nil {
		return err
	}
	if err != nil {
		return err
	}
	compose, err := cofingTmpl.Execute(gonja.Context{"operators": operators})
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

	// Generate BLS and ecdsa keys.
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

	// Read BLS and ECDSA keys.
	var blsKeyDir, ecdsaKeyDir string
	blsRegex, err := regexp.Compile("bls-")
	if err != nil {
		return err
	}
	ecdsaRegex, err := regexp.Compile("ecdsa-")
	if err != nil {
		return err
	}
	if err = filepath.Walk("./docker-compose/operators/keys", func(path string, info os.FileInfo, err error) error {
		if blsRegex.MatchString(info.Name()) {
			blsKeyDir = filepath.Join("./docker-compose/operators/keys", info.Name())
		}
		if ecdsaRegex.MatchString(info.Name()) {
			ecdsaKeyDir = filepath.Join("./docker-compose/operators/keys", info.Name())
		}
		return nil
	}); err != nil {
		return err
	}
	_, _, _, blsKeyPaths, err := readKeyDir(blsKeyDir)
	if err != nil {
		return err
	}
	_, _, ecdsaKeys, ecdsaKeyPaths, err := readKeyDir(ecdsaKeyDir)
	if err != nil {
		return err
	}

	// Generate configuration files for each operator.
	if _, err := os.Stat("./docker-compose/operators/configs"); os.IsNotExist(err) {
		if err = os.Mkdir("./docker-compose/operators/configs", os.ModePerm); err != nil {
			return err
		}
	} else if err != nil {
		if err = os.Remove("./docker-compose/operators/configs"); err != nil {
			return err
		}
	}
	ecdsaAddrs := make([]string, len(ecdsaKeys))
	for i, keyData := range ecdsaKeys {
		key := struct {
			Address string `json:"address"`
		}{}

		if err := json.Unmarshal(keyData, &key); err != nil {
			return err
		}
		ecdsaAddrs[i] = key.Address
	}
	for i := 0; i < int(operatorCount); i++ {
		operators[i] = map[string]interface{}{}
		cofingTmpl, err = gonja.FromFile("./docker-compose/operator-docker-compose.j2")
		if err != nil {
			return err
		}
		if err != nil {
			return err
		}
		config, err := cofingTmpl.Execute(gonja.Context{
			"address":        ecdsaAddrs[i],
			"ecdsa_key_path": ecdsaKeyPaths[i],
			"bls_key_path":   blsKeyPaths[i],
		})
		if err != nil {
			return err
		}
		configFile, err := os.Create(configPaths[i])
		if err != nil {
			return err
		}
		defer configFile.Close()
		if _, err = configFile.WriteString(config); err != nil {
			return err
		}
	}

	// Anvil snapshot.

	return nil
}

func readKeyDir(dirPath string) ([]string, []string, [][]byte, []string, error) {
	pwds, err := readFileLines(filepath.Join(dirPath, "password.txt"))
	if err != nil {
		return nil, nil, nil, nil, err
	}

	privKeys, err := readFileLines(filepath.Join(dirPath, "private_key_hex.txt"))
	if err != nil {
		return nil, nil, nil, nil, err
	}

	keys := make([][]byte, len(pwds))
	keyPaths := make([]string, len(keys))
	if err = filepath.Walk(
		filepath.Join(dirPath, "keys"),
		func(path string, info os.FileInfo, err error) error {
			if info.IsDir() {
				return nil
			}

			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			key, err := io.ReadAll(file)
			if err != nil {
				return err
			}

			keyIdxStr := strings.Split(info.Name(), ".")[0]
			keyIdx, err := strconv.ParseInt(keyIdxStr, 0, 32)
			if err != nil {
				return err
			}
			keys[keyIdx-1] = key
			keyPaths[keyIdx-1] = path

			return nil
		}); err != nil {
		return nil, nil, nil, nil, err
	}

	return pwds, privKeys, keys, keyPaths, nil
}

func readFileLines(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	lines := make([]string, 0)
	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return lines, nil
			} else {
				return nil, err
			}
		}
		lines = append(lines, string(line))
	}
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
