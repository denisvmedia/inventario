package shared

import (
	"fmt"
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

	// text, err := cleanenv.GetDescription(wrapper, nil)
	// if err == nil {
	//	fmt.Println(text)
	// }

	if err := cleanenv.ReadConfig(configFile, wrapper); err != nil {
		return cleanenv.ReadEnv(wrapper)
	}

	sectionValue := reflect.ValueOf(wrapper).Elem().Field(0)
	reflect.ValueOf(target).Elem().Set(sectionValue)

	return nil
}

func TryReadSection(sectionName string, target any) {
	if err := ReadSection(sectionName, target); err != nil {
		// ignore error, use defaults
	}
}

func ReadVirtualSection(sectionName string, target any) error {
	sectionType := reflect.TypeOf(target).Elem()
	wrapperType := reflect.StructOf([]reflect.StructField{
		{
			Name: "Section",
			Type: sectionType,
			Tag:  reflect.StructTag(fmt.Sprintf(`yaml:",inline" env-prefix:"%s_%s_"`, envPrefix, sectionName)),
		},
	})

	wrapper := reflect.New(wrapperType).Interface()
	if err := cleanenv.ReadConfig(configFile, wrapper); err != nil {
		return cleanenv.ReadEnv(wrapper)
	}

	sectionValue := reflect.ValueOf(wrapper).Elem().Field(0)
	reflect.ValueOf(target).Elem().Set(sectionValue)

	return nil
}

func TryReadVirtualSection(sectionName string, target any) {
	if err := ReadVirtualSection(sectionName, target); err != nil {
		// ignore error, use defaults
	}
}
