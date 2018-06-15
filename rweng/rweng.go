package rweng

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-yaml/yaml"
	"go.uber.org/zap"
)

// EngCfg defines an engine configuration
type EngCfg struct {
	PostBan []string `yaml:"postBan"`
	UrlBan  []string `yaml:"urlBan"`
}

// Eng http.Request rule engine.
type Eng struct {
	cfg     EngCfg
	postBan []*regexp.Regexp
	urlBan  []*regexp.Regexp
	logger  *zap.Logger
}

// ProcessRequest performs any rules on matching requests
func (e *Eng) ProcessRequest(w http.ResponseWriter, r *http.Request) {

	b, _ := ioutil.ReadAll(r.Body)
	r.Body.Close()

	b = bytes.Replace(b, []byte("fun"), []byte("euQQ3i8J"), -1)

	// search for qstring contraband
	for _, rgx := range e.urlBan {
		buri := bytes.ToLower([]byte(r.RequestURI))
		if rgx.Match(buri) {
			e.logger.Info(fmt.Sprintf("URL contraband found [%s] in:\n%s\n", rgx, buri))
			r.URL.Path = "/"
			r.URL.RawQuery = ""
			break
		}
	}

	// search for posted contraband
	for _, rgx := range e.postBan {
		if rgx.Match(bytes.ToLower(b)) {
			e.logger.Info(fmt.Sprintf("Posted contraband found [%s] in:\n%s\n", rgx, b))
			b = []byte{}
			break
		}
	}

	body := ioutil.NopCloser(bytes.NewReader(b))

	r.Body = body
	r.ContentLength = int64(len(b))
	r.Header.Set("Content-Length", strconv.Itoa(len(b)))

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

	postBan := make([]*regexp.Regexp, 0)

	for _, r := range engCfg.PostBan {
		rxp := regexp.MustCompile(strings.ToLower(r))
		postBan = append(postBan, rxp)
	}

	urlBan := make([]*regexp.Regexp, 0)

	for _, r := range engCfg.UrlBan {
		rxp := regexp.MustCompile(strings.ToLower(r))
		urlBan = append(urlBan, rxp)
	}

	eng := &Eng{
		cfg:     engCfg,
		postBan: postBan,
		urlBan:  urlBan,
		logger:  logger,
	}

	return eng, nil
}
