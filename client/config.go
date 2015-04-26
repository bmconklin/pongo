package client

import (
    "os"
    "errors"
    "reflect"
    "runtime"
    "encoding/json"
)

type Config struct {
}

func (c *Config) validate() bool {
    return true
}

func GetConfig(filepath string) (*Config, error) {
    var config *Config 
    file, err := os.Open(filepath)
    if err != nil {
        return nil, errors.New("Error: Config file not found. Please check " + filepath)
    }
    defer file.Close()

    jsonDecoder := json.NewDecoder(file)
    if err = jsonDecoder.Decode(&config); err != nil {
        return nil, errors.New("Unable to decode config file. " + err.Error())
    }

    // validate before allowing to continue
    if !config.validate() {
        return nil, errors.New("Validation of config.json failed, exiting.")
    }

    return config, nil
}

func setCPUs(cpuValue interface{}) error {
    switch reflect.ValueOf(cpuValue).Kind().String() {
    case "string":
        if reflect.ValueOf(cpuValue).String() == "max" {
            runtime.GOMAXPROCS(runtime.NumCPU())
            return nil
        } else {
            return errors.New("Unrecognized string in cpus field of config.json")
        }
    case "int": 
        runtime.GOMAXPROCS(int(reflect.ValueOf(cpuValue).Int()))
        return nil
    }
    return errors.New("Unrecognized value in cpus field of config.json")
}
