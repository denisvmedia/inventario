package shared

import (
	"fmt"
	"log/slog"
	"reflect"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
)

var envPrefix = "INVENTARIO"

func SetEnvPrefix(prefix string) {
	envPrefix = prefix
}

var configFile = "config.yaml"

func SetConfigFile(file string) {
	configFile = file
}

func GetConfigFile() string {
	return configFile
}

func ReadSection(sectionName string, target any) error {
	replacer := strings.NewReplacer(".", "_", "-", "_")
	envPrefixFull := fmt.Sprintf("%s_%s_", envPrefix, strings.ToUpper(sectionName))
	tag := fmt.Sprintf(`yaml:"%s" env-prefix:"%s"`, sectionName, replacer.Replace(envPrefixFull))
	slog.Debug("Reading section", "section", sectionName, "tag", tag)
	sectionType := reflect.TypeOf(target).Elem()
	wrapperType := reflect.StructOf([]reflect.StructField{
		{
			Name: "Section",
			Type: sectionType,
			Tag:  reflect.StructTag(tag),
		},
	})

	wrapper := reflect.New(wrapperType).Interface()

	if err := cleanenv.ReadConfig(configFile, wrapper); err != nil {
		if err := cleanenv.ReadEnv(wrapper); err != nil {
			slog.Error("Failed to read config", "error", err)
		}
	}

	sectionValue := reflect.ValueOf(wrapper).Elem().Field(0)
	reflect.ValueOf(target).Elem().Set(sectionValue)

	return nil
}

func TryReadSection(sectionName string, target any) {
	_ = ReadSection(sectionName, target)
}

func ReadVirtualSection(sectionName string, target any) error {
	replacer := strings.NewReplacer(".", "_", "-", "_")
	var envPrefixFull string
	if sectionName == "." || sectionName == "" {
		envPrefixFull = envPrefix + "_"
	} else {
		envPrefixFull = fmt.Sprintf("%s_%s_", envPrefix, strings.ToUpper(sectionName))
	}
	tag := fmt.Sprintf(`yaml:",inline" env-prefix:"%s"`, replacer.Replace(envPrefixFull))
	slog.Debug("Reading virtual section", "section", sectionName, "tag", tag)
	sectionType := reflect.TypeOf(target).Elem()
	wrapperType := reflect.StructOf([]reflect.StructField{
		{
			Name: "Section",
			Type: sectionType,
			Tag:  reflect.StructTag(tag),
		},
	})

	wrapper := reflect.New(wrapperType).Interface()
	if err := cleanenv.ReadConfig(configFile, wrapper); err != nil {
		if err = cleanenv.ReadEnv(wrapper); err != nil {
			slog.Error("Failed to read config", "error", err)
		} else {
			slog.Debug("Read config from environment variables")
		}
	} else {
		slog.Debug("Read config from file")
	}

	sectionValue := reflect.ValueOf(wrapper).Elem().Field(0)
	reflect.ValueOf(target).Elem().Set(sectionValue)

	return nil
}

func TryReadVirtualSection(sectionName string, target any) {
	_ = ReadVirtualSection(sectionName, target)
}
