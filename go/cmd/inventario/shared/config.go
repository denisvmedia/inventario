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
	sectionType := reflect.TypeOf(target).Elem()
	wrapperType := reflect.StructOf([]reflect.StructField{
		{
			Name: "Section",
			Type: sectionType,
			Tag:  reflect.StructTag(fmt.Sprintf(`yaml:"%s" env-prefix:"%s_%s_"`, sectionName, envPrefix, strings.ToUpper(sectionName))),
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
	sectionType := reflect.TypeOf(target).Elem()
	wrapperType := reflect.StructOf([]reflect.StructField{
		{
			Name: "Section",
			Type: sectionType,
			Tag:  reflect.StructTag(fmt.Sprintf(`yaml:",inline" env-prefix:"%s_%s_"`, envPrefix, strings.ToUpper(sectionName))),
		},
	})

	wrapper := reflect.New(wrapperType).Interface()
	if err := cleanenv.ReadConfig(configFile, wrapper); err != nil {
		if err = cleanenv.ReadEnv(wrapper); err != nil {
			slog.Error("Failed to read config", "error", err)
		}
	}

	sectionValue := reflect.ValueOf(wrapper).Elem().Field(0)
	reflect.ValueOf(target).Elem().Set(sectionValue)

	return nil
}

func TryReadVirtualSection(sectionName string, target any) {
	_ = ReadVirtualSection(sectionName, target)
}
