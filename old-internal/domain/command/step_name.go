package command

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
)

type StepNameValue string

const (
	StepTest    StepNameValue = "test"
	StepSupply  StepNameValue = "supply"
	StepPackage StepNameValue = "package"
	StepDeploy  StepNameValue = "deploy"
)

// StepDirNameFormat describe el formato del directorio de un step, para los
// mensajes de error que lo nombran.
const StepDirNameFormat = "NN-nombre con exactamente dos dígitos"

// stepNameRegex exige EXACTAMENTE dos dígitos. No es una preferencia estética:
// `FullName()` formatea con `%02d`, así que con `\d+` un directorio `2-supply`
// producía un StepName cuyo `FullName()` era `02-supply` — el motor buscaba un
// `commands.yaml` que no existía y compartía la clave de estado con un
// pipelinecode distinto (spec 04 §5.1).
var stepNameRegex = regexp.MustCompile(`^(\d{2})-(.+)$`)

type StepName struct {
	order int
	name  string
}

func NewStepName(dirName string) (StepName, error) {
	matches := stepNameRegex.FindStringSubmatch(dirName)
	if len(matches) != 3 {
		return StepName{}, fmt.Errorf("el nombre del directorio del paso '%s' no sigue el formato '%s'", dirName, StepDirNameFormat)
	}

	order, err := strconv.Atoi(matches[1])
	if err != nil {
		return StepName{}, fmt.Errorf("no se pudo parsear el número de orden del paso '%s'", dirName)
	}

	name := matches[2]
	if name == "" {
		return StepName{}, errors.New("el nombre del paso no puede estar vacío")
	}

	return StepName{order: order, name: name}, nil
}

func (s StepName) Order() int {
	return s.order
}

func (s StepName) Name() string {
	return s.name
}

// FullName es el nombre canónico del step: el del directorio del que salió y la
// clave con la que se persiste su estado. Con el prefijo validado a dos dígitos
// `%02d` es la identidad —reformatea a lo que ya era—, así que no normaliza
// nada; se conserva porque es el formato canónico (spec 04 §8).
func (s StepName) FullName() string {
	return fmt.Sprintf("%02d-%s", s.order, s.name)
}
