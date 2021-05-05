package main

import (
	"fmt"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cmdrunner"
	"github.com/jenkins-x/jx-helpers/v3/pkg/files"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient/cli"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient/giturl"
	"github.com/jenkins-x/jx-helpers/v3/pkg/stringhelpers"
	"github.com/jenkins-x/jx-helpers/v3/pkg/termcolor"
	"github.com/jenkins-x/jx-logging/v3/pkg/log"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ResourceSchema struct {
	APIVersion string
	Name       string
	URL        string
}

var (
	defaultKubevalVersion = "v1.18.1-standalone"

	// repo the git repositories to clone to find the schema docs
	repos = []string{
		"https://github.com/jenkins-x/jx-api",
		"https://github.com/jenkins-x-plugins/jx-charter",
		"https://github.com/jenkins-x-plugins/jx-preview",
	}

	cloneRepositories = os.Getenv("JX_DISABLE_GIT_CLONE") != "true"

	info = termcolor.ColorInfo

	template = `
## Jenkins X JSON Schema files

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)


### Schemas


| API Version | Kind |
| --- | --- |
`
)

func main() {
	o := &Options{}
	if len(os.Args) > 1 {
		o.Dir = os.Args[1]
	}
	err := o.Run()
	if err != nil {
		log.Logger().Errorf("failed: %v", err)
		os.Exit(1)
	}
	log.Logger().Infof("completed the plugin generator")
	os.Exit(0)
}

type Options struct {
	Dir            string
	WorkDir        string
	KubeValVersion string
	Schemas        []ResourceSchema
	GitClient      gitclient.Interface
	CommandRunner  cmdrunner.CommandRunner
}

func (o *Options) Run() error {
	err := o.Validate()
	if err != nil {
		return errors.Wrapf(err, "failed to validate options")
	}

	err = o.cloneRepositories()
	if err != nil {
		return errors.Wrapf(err, "failed to clone plugins")
	}

	err = o.generateIndex(o.Schemas)
	if err != nil {
		return errors.Wrapf(err, "failed to generate schema index")
	}

	log.Logger().Infof("completed")
	return nil
}

// Validate validates the setup
func (o *Options) Validate() error {
	if o.CommandRunner == nil {
		o.CommandRunner = cmdrunner.QuietCommandRunner
	}
	if o.GitClient == nil {
		o.GitClient = cli.NewCLIClient("", o.CommandRunner)
	}
	if o.Dir == "" {
		o.Dir = "."
	}
	if o.WorkDir == "" {
		o.WorkDir = filepath.Join(o.Dir, "jx-repos")
	}
	if o.KubeValVersion == "" {
		o.KubeValVersion = os.Getenv("KUBEVAL_VERSION")
		if o.KubeValVersion == "" {
			o.KubeValVersion = defaultKubevalVersion
		}
	}
	log.Logger().Infof("using directory %s", info(o.WorkDir))
	err := os.MkdirAll(o.WorkDir, files.DefaultDirWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to create dir %s", o.WorkDir)
	}
	return nil
}

func (o *Options) cloneRepositories() error {
	dir, err := filepath.Abs(o.WorkDir)
	if err != nil {
		return errors.Wrapf(err, "failed to get absolute dir of %s", o.WorkDir)
	}

	err = os.RemoveAll(dir)
	if err != nil {
		return errors.Wrapf(err, "failed to remove dir %s", dir)
	}
	err = os.MkdirAll(dir, files.DefaultDirWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to create dir %s", dir)
	}

	for _, u := range repos {
		gitInfo, err := giturl.ParseGitURL(u)
		if err != nil {
			return errors.Wrapf(err, "failed to parse git URL %s", u)
		}
		n := gitInfo.Name
		err = o.cloneRepository(n, u, dir)
		if err != nil {
			return errors.Wrapf(err, "failed to clone repository")
		}
	}

	err = o.generateKubevalFiles(o.Schemas)
	if err != nil {
		return errors.Wrapf(err, "failed to generate kubeval files")
	}

	err = gitclient.Add(o.GitClient, o.Dir, "docs")
	if err != nil {
		return errors.Wrapf(err, "failed to add to git")
	}

	err = gitclient.CommitIfChanges(o.GitClient, o.Dir, "chore: regenerate schema docs")
	if err != nil {
		return errors.Wrapf(err, "failed to commit changes")
	}
	return nil
}

func (o *Options) cloneRepository(name, gitURL, dir string) error {
	if gitURL == "" {
		log.Logger().Warnf("no clone URL for repository %s", name)
		return nil
	}

	toDir := filepath.Join(dir, name)
	err := os.MkdirAll(toDir, files.DefaultDirWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to create dir %s", toDir)
	}

	log.Logger().Infof("cloning plugin %s to %s ", info(name), info(toDir))
	_, err = gitclient.CloneToDir(o.GitClient, gitURL, toDir)
	if err != nil {
		return errors.Wrapf(err, "failed to clone %s to %s", gitURL, toDir)
	}

	return o.generateDocs(toDir)
}

func (o *Options) generateDocs(toDir string) error {
	log.Logger().Infof("reading %s", info(toDir))

	dir := filepath.Join(toDir, "schema")
	exists, err := files.DirExists(dir)
	if err != nil {
		return errors.Wrapf(err, "failed to check if schema dir exists %s", dir)
	}
	if !exists {
		log.Logger().Warnf("no schema dir for repository")
		return nil
	}

	// lets iterate through all files for schema files..
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info == nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".json") {
			return nil
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return errors.Wrapf(err, "failed to get relative path of %s within %s", path, dir)
		}

		dest := filepath.Join(o.Dir, "docs", rel)
		destDir := filepath.Dir(dest)
		err = os.MkdirAll(destDir, files.DefaultDirWritePermissions)
		if err != nil {
			return errors.Wrapf(err, "failed to create dir %s", destDir)
		}
		err = files.CopyFile(path, dest)
		if err != nil {
			return errors.Wrapf(err, "failed to copy file %s to %s", path, dest)
		}
		log.Logger().Debugf("copied file %s", termcolor.ColorInfo(path))

		relDir, relName := filepath.Split(rel)
		relDir = strings.TrimSuffix(relDir, string(os.PathSeparator))

		kindName := strings.TrimSuffix(relName, ".json")
		kindName = strings.Title(stringhelpers.ToCamelCase(kindName))

		o.Schemas = append(o.Schemas, ResourceSchema{
			APIVersion: relDir,
			Name:       kindName,
			URL:        rel,
		})
		return nil
	})
	if err != nil {
		return errors.Wrapf(err, "failed to copy schema files")
	}
	return nil
}

func (o *Options) generateKubevalFiles(schemas []ResourceSchema) error {
	for _, sch := range schemas {
		log.Logger().Infof("schema %#v", sch)

		paths := strings.Split(sch.APIVersion, "/")
		if len(paths) < 2 {
			log.Logger().Infof("ignoring schema version with no version %s", sch.APIVersion)
			continue
		}
		names := strings.Split(paths[0], ".")
		name := names[0]
		version := paths[len(paths)-1]
		dest := filepath.Join(o.Dir, "docs", o.KubeValVersion, strings.ToLower(sch.Name)+"-"+name+"-"+version+".json")
		src := filepath.Join(o.Dir, "docs", sch.URL)

		err := files.CopyFile(src, dest)
		if err != nil {
			return errors.Wrapf(err, "failed to copy %s to %s", src, dest)
		}
	}
	return nil
}

func (o *Options) generateIndex(schemas []ResourceSchema) error {
	buf := &strings.Builder{}

	sort.Slice(schemas, func(i, j int) bool {
		s1 := schemas[i]
		s2 := schemas[j]

		if s1.APIVersion != s2.APIVersion {
			return s1.APIVersion < s2.APIVersion
		}
		return s1.Name < s2.Name
	})

	buf.WriteString(template)
	for _, s := range schemas {
		buf.WriteString(fmt.Sprintf("| [%s](%s) | [%s](%s) |\n", s.APIVersion, s.APIVersion, s.Name, s.URL))
	}

	text := buf.String()

	path := filepath.Join(o.Dir, "docs", "README.md")
	err := ioutil.WriteFile(path, []byte(text), files.DefaultFileWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to save file %s", path)
	}
	log.Logger().Infof("created index file %s", info(path))
	return nil
}
