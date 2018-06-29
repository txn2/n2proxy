package rweng

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"os"

	"github.com/Masterminds/sprig"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

type FilterCfg struct {
	Name     string `yaml:"name"`
	Match    string `yaml:"match"`
	Template string `yaml:"template"`
}

type FilterTemplate struct {
	Name     string
	Match    string
	Template *template.Template
}

// EngCfg defines an engine configuration
type EngCfg struct {
	UrlWhiteList []string    `yaml:"urlWhiteList"`
	PostBan      []string    `yaml:"postBan"`
	UrlBan       []string    `yaml:"urlBan"`
	QueryBan     []string    `yaml:"queryBan"`
	Filter       []FilterCfg `yaml:"postFilter"`
}

// Eng http.Request rule engine.
type Eng struct {
	cfg          EngCfg
	urlWhiteList []*regexp.Regexp
	postBan      []*regexp.Regexp
	urlBan       []*regexp.Regexp
	queryBan     []*regexp.Regexp
	filter       map[*regexp.Regexp]FilterTemplate
	logger       *zap.Logger
}

// ProcessRequest performs any rules on matching requests
func (e *Eng) ProcessRequest(w http.ResponseWriter, r *http.Request) {

	b, _ := ioutil.ReadAll(r.Body)
	r.Body.Close()

	// bypass on urlWhitelist
	for _, rgx := range e.urlWhiteList {
		buri := bytes.ToLower([]byte(r.RequestURI))
		if rgx.Match(buri) {
			e.logger.Warn("Bypassing: Whitelisted URL found.", zap.String("Regexp", rgx.String()), zap.ByteString("URI", buri))
			return
		}
	}

	// run filter if there is a body
	if len(b) > 0 {
		for rgx, filter := range e.filter {

			// find the match first and populate data structure
			matches := rgx.FindAll(bytes.ToLower(b), len(b))
			for _, match := range matches {
				filter.Match = string(match)
				// send the match to the template
				var tplReturn bytes.Buffer
				if err := filter.Template.Execute(&tplReturn, filter); err != nil {
					// something bad happened
					e.logger.Error("Filter failed: " + err.Error())
					continue
				}

				b = rgx.ReplaceAll(b, tplReturn.Bytes())
			}
		}
	}

	// search for url path contraband
	for _, rgx := range e.urlBan {
		buri := bytes.ToLower([]byte(r.RequestURI))
		if rgx.Match(buri) {
			e.logger.Warn("URL contraband found.", zap.String("Regexp", rgx.String()), zap.ByteString("URI", buri))
			r.URL.Path = "/"
			r.URL.RawQuery = ""
			break
		}
	}

	if len(r.URL.RawQuery) > 0 {
		// search for url path contraband
		for _, rgx := range e.queryBan {
			bq := bytes.ToLower([]byte(r.URL.RawQuery))
			if rgx.Match(bq) {
				e.logger.Warn("QUERY STRING contraband found.", zap.String("Regexp", rgx.String()), zap.ByteString("QUERY", bq))
				r.URL.Path = "/"
				r.URL.RawQuery = ""
				break
			}
		}
	}

	// search for posted contraband
	for _, rgx := range e.postBan {
		if rgx.Match(bytes.ToLower(b)) {
			e.logger.Warn("Posted contraband found.", zap.String("Regexp", rgx.String()), zap.ByteString("PostBody", b))
			b = []byte{}
			break
		}
	}

	body := ioutil.NopCloser(bytes.NewReader(b))

	r.Body = body
	r.ContentLength = int64(len(b))
	r.Header.Set("Content-Length", strconv.Itoa(len(b)))

}

// regexpCompile
func regexpCompile(rrxp []string) ([]*regexp.Regexp, error) {
	crxp := make([]*regexp.Regexp, 0)

	for _, r := range rrxp {
		rxp, err := regexp.Compile("(?i)" + strings.ToLower(r))
		if err != nil {
			return crxp, err
		}
		crxp = append(crxp, rxp)
	}

	return crxp, nil
}

// NewEngFromYml loads an engine from yaml data
func NewEngFromYml(filename string, logger *zap.Logger) (*Eng, error) {

	ymlData, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	engCfg := EngCfg{}

	err = yaml.Unmarshal([]byte(ymlData), &engCfg)
	if err != nil {
		return nil, err
	}

	urlWhileList, err := regexpCompile(engCfg.UrlWhiteList)
	if err != nil {
		logger.Error("Error in urlWhileList regex compile: " + err.Error())
		os.Exit(1)
	}

	postBan, err := regexpCompile(engCfg.PostBan)
	if err != nil {
		logger.Error("Error in postBan regex compile: " + err.Error())
		os.Exit(1)
	}

	urlBan, err := regexpCompile(engCfg.UrlBan)
	if err != nil {
		logger.Error("Error in urlBan regex compile: " + err.Error())
		os.Exit(1)
	}

	queryBan, err := regexpCompile(engCfg.QueryBan)
	if err != nil {
		logger.Error("Error in queryBan regex compile: " + err.Error())
		os.Exit(1)
	}

	filter := make(map[*regexp.Regexp]FilterTemplate, 0)

	for _, filterCfg := range engCfg.Filter {
		rxp, err := regexp.Compile(strings.ToLower(filterCfg.Match))
		if err != nil {
			logger.Error("Error in filterCfg regex compile: " + err.Error())
			os.Exit(1)
		}

		tmpl, err := template.New(filterCfg.Name).Funcs(sprig.TxtFuncMap()).Parse(filterCfg.Template)
		if err != nil {
			logger.Error("Template parsing error: " + err.Error())
		}

		filter[rxp] = FilterTemplate{
			Name:     filterCfg.Name,
			Template: tmpl,
		}
	}

	eng := &Eng{
		cfg:          engCfg,
		urlWhiteList: urlWhileList,
		postBan:      postBan,
		urlBan:       urlBan,
		queryBan:     queryBan,
		filter:       filter,
		logger:       logger,
	}

	return eng, nil
}
