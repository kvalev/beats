package template

import (
	"fmt"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/paths"
)

// TemplateLoader is a subset of the Elasticsearch client API capable of
// loading the template.
type ESClient interface {
	LoadJSON(path string, json map[string]interface{}) ([]byte, error)
	Request(method, path string, pipeline string, params map[string]string, body interface{}) (int, []byte, error)
	GetVersion() string
}

type Loader struct {
	config   TemplateConfig
	client   ESClient
	beatInfo beat.Info
	fields   []byte
}

// NewLoader creates a new template loader
func NewLoader(cfg *common.Config, client ESClient, beatInfo beat.Info, fields []byte) (*Loader, error) {
	config := DefaultConfig

	err := cfg.Unpack(&config)
	if err != nil {
		return nil, err
	}

	return &Loader{
		config:   config,
		client:   client,
		beatInfo: beatInfo,
		fields:   fields,
	}, nil
}

// Load checks if the index mapping template should be loaded
// In case the template is not already loaded or overwriting is enabled, the
// template is written to index
func (l *Loader) Load() error {

	tmpl, err := New(l.beatInfo.Version, l.beatInfo.IndexPrefix, l.client.GetVersion(), l.config)
	if err != nil {
		return fmt.Errorf("error creating template instance: %v", err)
	}

	// Check if template already exist or should be overwritten
	exists := l.CheckTemplate(tmpl.GetName())
	if !exists || l.config.Overwrite {

		logp.Info("Loading template for Elasticsearch version: %s", l.client.GetVersion())

		if l.config.Overwrite {
			logp.Info("Existing template will be overwritten, as overwrite is enabled.")
		}

		var output common.MapStr

		// Load fields from path
		if l.config.Fields != "" {
			logp.Debug("template", "Load fields.yml from file: %s", l.config.Fields)

			fieldsPath := paths.Resolve(paths.Config, l.config.Fields)

			output, err = tmpl.LoadFile(fieldsPath)
			if err != nil {
				return fmt.Errorf("error creating template from file %s: %v", fieldsPath, err)
			}
		} else {
			logp.Debug("template", "Load default fields.yml")
			output, err = tmpl.LoadBytes(l.fields)
			if err != nil {
				return fmt.Errorf("error creating template: %v", err)
			}
		}

		err = l.LoadTemplate(tmpl.GetName(), output)
		if err != nil {
			return fmt.Errorf("could not load template. Elasticsearh returned: %v. Template is: %s", err, output)
		}
	} else {
		logp.Info("Template already exists and will not be overwritten.")
	}

	return nil
}

// LoadTemplate loads a template into Elasticsearch overwriting the existing
// template if it exists. If you wish to not overwrite an existing template
// then use CheckTemplate prior to calling this method.
func (l *Loader) LoadTemplate(templateName string, template map[string]interface{}) error {
	logp.Debug("template", "Try loading template with name: %s", templateName)
	path := "/_template/" + templateName
	body, err := l.client.LoadJSON(path, template)
	if err != nil {
		return fmt.Errorf("couldn't load template: %v. Response body: %s", err, body)
	}
	logp.Info("Elasticsearch template with name '%s' loaded", templateName)
	return nil
}

// CheckTemplate checks if a given template already exist. It returns true if
// and only if Elasticsearch returns with HTTP status code 200.
func (l *Loader) CheckTemplate(templateName string) bool {
	status, _, _ := l.client.Request("HEAD", "/_template/"+templateName, "", nil, nil)

	if status != 200 {
		return false
	}

	return true
}
