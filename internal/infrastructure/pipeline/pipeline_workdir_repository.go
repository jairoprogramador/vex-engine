package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	domPipeline "github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/persistence"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/utils"
)

const gitDirName = ".git"

const (
	// copiesDirName es donde vive el inventario de la última copia. Va FUERA del
	// workdir a propósito: un archivo dentro sería una plantilla más que
	// interpolar, y la corrida siguiente lo trataría como residuo del
	// pipelinecode.
	copiesDirName = ".copias"

	copyManifestExt = ".json"

	// fileCopyManifestSchemaVersion versiona la forma del inventario.
	fileCopyManifestSchemaVersion = 1
)

// FileCopyManifestDTO es lo que ESTA copia puso en el workdir.
//
// Existe por la poda de la spec 18 §5.4: `Copy` sobrescribía y nunca borraba, así
// que un archivo retirado del pipelinecode sobrevivía en la copia de trabajo con
// su último contenido interpolado. Con la ventana de reutilización del clon ese
// residuo pasa a cruzar ejecuciones de verdad, porque el clon ya no se rehace.
type FileCopyManifestDTO struct {
	SchemaVersion int      `json:"schema_version"`
	Files         []string `json:"files"`
}

var _ domPipeline.PipelineWorkdirRepository = (*PipelineWorkdirRepository)(nil)

type PipelineWorkdirRepository struct {
	workdirBasePath string
	writer          persistence.AtomicFileWriter
}

func NewPipelineWorkdirRepository(workdirBasePath string) domPipeline.PipelineWorkdirRepository {
	return &PipelineWorkdirRepository{
		workdirBasePath: workdirBasePath,
		writer:          persistence.NewAtomicFileWriter(),
	}
}

func (r *PipelineWorkdirRepository) destinationDir(projectUrl, pipelineUrl, environment string) string {
	projectName := utils.GetDirNameFromUrl(projectUrl)
	pipelineName := utils.GetDirNameFromUrl(pipelineUrl)
	return filepath.Join(r.workdirBasePath, projectName, "workdirs", pipelineName, environment)
}

func (r *PipelineWorkdirRepository) Copy(ctx *context.Context, localPipelinePath, projectUrl, pipelineUrl, environment string) (string, error) {
	sourceInfo, err := os.Stat(localPipelinePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("pipeline workdir repository: la ruta de origen no existe: %s", localPipelinePath)
		}
		return "", fmt.Errorf("pipeline workdir repository: stat origen '%s': %w", localPipelinePath, err)
	}
	if !sourceInfo.IsDir() {
		return "", fmt.Errorf("pipeline workdir repository: la ruta '%s' no es un directorio", localPipelinePath)
	}

	destDir := r.destinationDir(projectUrl, pipelineUrl, environment)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("pipeline workdir repository: crear directorio destino '%s': %w", destDir, err)
	}

	sourceMode := sourceInfo.Mode()
	copiados := make([]string, 0, 32)

	err = filepath.WalkDir(localPipelinePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		select {
		case <-(*ctx).Done():
			return (*ctx).Err()
		default:
		}

		if d.Name() == gitDirName {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, err := filepath.Rel(localPipelinePath, path)
		if err != nil {
			return fmt.Errorf("pipeline workdir repository: rel path '%s': %w", path, err)
		}
		if relPath == "." {
			return nil
		}

		destPath := filepath.Join(destDir, relPath)

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("pipeline workdir repository: info '%s': %w", path, err)
		}

		if d.IsDir() {
			if err := os.MkdirAll(destPath, info.Mode()); err != nil {
				return fmt.Errorf("pipeline workdir repository: mkdir '%s': %w", destPath, err)
			}
			return nil
		}

		if err := os.MkdirAll(filepath.Dir(destPath), sourceMode); err != nil {
			return fmt.Errorf("pipeline workdir repository: mkdir padre '%s': %w", filepath.Dir(destPath), err)
		}
		if err := copyFile(path, destPath, info.Mode()); err != nil {
			return fmt.Errorf("pipeline workdir repository: copiar '%s' -> '%s': %w", path, destPath, err)
		}
		copiados = append(copiados, filepath.ToSlash(relPath))
		return nil
	})
	if err != nil {
		return "", err
	}

	if err := r.podar(destDir, projectUrl, pipelineUrl, environment, copiados); err != nil {
		return "", err
	}

	return destDir, nil
}

// podar borra del workdir lo que la copia ANTERIOR puso y ésta ya no pone, y
// deja anotado lo que ésta puso.
//
// # Por qué no se poda «todo lo que no está en el origen»
//
// Porque el workdir no sólo tiene la copia del pipelinecode: es también donde
// CORREN los comandos que declaran un `workdir`, así que ahí dentro aparece el
// `.terraform/` de un `terraform init` o el `target/` de un build. Eso no es
// residuo, es trabajo, y borrarlo convertiría una limpieza en una pérdida.
//
// Lo que sí es residuo está definido con precisión: un archivo que la copia
// anterior escribió y que el pipelinecode ya no trae. Para saberlo hay que
// recordar qué se escribió, y por eso hay un inventario.
//
// # Y por qué la poda pasa a hacer falta ahora
//
// Hasta la spec 18 la limpieza del workdir estaba DOBLEMENTE asegurada: la
// spec 06 restaura las plantillas, y además el clonador hacía `os.RemoveAll` en
// cada corrida. La ventana de reutilización retira la segunda red (§5.4), así
// que el residuo pasa a cruzar ejecuciones de verdad.
//
// No poder escribir el inventario NO tumba la ejecución: la copia está hecha y
// es correcta; lo que se pierde es la poda de la corrida siguiente. Fallar aquí
// convertiría un despliegue bueno en uno fallido por un archivo auxiliar.
func (r *PipelineWorkdirRepository) podar(
	destDir, projectUrl, pipelineUrl, environment string, copiados []string) error {

	anteriores := r.leerInventario(projectUrl, pipelineUrl, environment)

	actuales := make(map[string]bool, len(copiados))
	for _, rel := range copiados {
		actuales[rel] = true
	}

	sobrantes := make([]string, 0)
	for _, rel := range anteriores {
		if !actuales[rel] {
			sobrantes = append(sobrantes, rel)
		}
	}

	for _, rel := range sobrantes {
		path := filepath.Join(destDir, filepath.FromSlash(rel))
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("pipeline workdir repository: podar '%s': %w", path, err)
		}
		// El directorio que se queda vacío también sobra. Se intenta y se ignora
		// el fallo: un directorio con algo dentro es un directorio que alguien
		// más está usando, y ése no es residuo.
		removerDirectoriosVacios(destDir, filepath.Dir(path))
	}

	return r.escribirInventario(projectUrl, pipelineUrl, environment, copiados)
}

func (r *PipelineWorkdirRepository) manifestPath(projectUrl, pipelineUrl, environment string) string {
	destDir := r.destinationDir(projectUrl, pipelineUrl, environment)
	return filepath.Join(filepath.Dir(destDir), copiesDirName, environment+copyManifestExt)
}

// leerInventario devuelve lo que la copia anterior puso. Un inventario ausente o
// ilegible es «no se sabe», y no se sabe ⇒ no se poda: borrar por una suposición
// es exactamente lo que no puede hacer una limpieza.
func (r *PipelineWorkdirRepository) leerInventario(
	projectUrl, pipelineUrl, environment string) []string {

	data, err := os.ReadFile(r.manifestPath(projectUrl, pipelineUrl, environment))
	if err != nil {
		return nil
	}
	var dto FileCopyManifestDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		return nil
	}
	if dto.SchemaVersion != fileCopyManifestSchemaVersion {
		return nil
	}
	return dto.Files
}

func (r *PipelineWorkdirRepository) escribirInventario(
	projectUrl, pipelineUrl, environment string, copiados []string) error {

	sort.Strings(copiados)
	dto := FileCopyManifestDTO{
		SchemaVersion: fileCopyManifestSchemaVersion,
		Files:         copiados,
	}

	path := r.manifestPath(projectUrl, pipelineUrl, environment)
	err := r.writer.Write(path, func(out io.Writer) error {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(dto)
	})
	if err != nil {
		return fmt.Errorf("pipeline workdir repository: escribir el inventario %s: %w", path, err)
	}
	return nil
}

// removerDirectoriosVacios sube desde `dir` hasta `root` borrando lo que quede
// vacío. Nunca borra `root`, que es el workdir.
func removerDirectoriosVacios(root, dir string) {
	root = filepath.Clean(root)
	for dir = filepath.Clean(dir); dir != root && len(dir) > len(root); dir = filepath.Dir(dir) {
		if err := os.Remove(dir); err != nil {
			return
		}
	}
}

func copyFile(src, dst string, perm fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
