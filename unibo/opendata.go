package unibo

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

const (
	rootUnibo = "https://dati.unibo.it"
)

var (
	reg    = regexp.MustCompile(`<a title="Sito del corso" href="https://corsi\.unibo\.it/(.+?)"`)
	Client = http.Client{
		Transport: &transport{
			http.DefaultTransport,
		},
	}
)

type transport struct {
	http.RoundTripper
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", "CalendarBot")
	return t.RoundTripper.RoundTrip(req)
}

type Package struct {
	Success bool `json:"success"`
	Result  struct {
		Resources Resources
	}
}

type Resources []Resource

func (r Resources) GetByAlias(alias string) *Resource {
	for _, resource := range r {
		rAliases := strings.Split(resource.Alias, ", ")
		for _, rAlias := range rAliases {
			if rAlias == alias {
				return &resource
			}
		}
	}
	return nil
}

type Resource struct {
	Frequency string `json:"frequency"`
	Url       string `json:"url"`
	Id        string `json:"id"`
	PackageId string `json:"package_id"`
	LastMod   string `json:"last_modified"`
	Alias     string `json:"alias"`
}

func (r Resource) DownloadCourses() ([]Course, error) {
	if strings.HasSuffix(r.Url, ".csv") {
		req, err := Client.Get(r.Url)
		if err != nil {
			return nil, err
		}

		reader := csv.NewReader(req.Body)
		if err != nil {
			return nil, err
		}

		courses := make([]Course, 0, 100)

		// Skip first line
		_, err = reader.Read()
		if err != nil {
			return nil, err
		}

		for {
			row, err := reader.Read()
			if err != nil {
				if err == io.EOF {
					break
				} else {
					return nil, err
				}
			}

			code, err := strconv.ParseInt(row[2], 10, 32)
			if err != nil {
				return nil, err
			}

			years, err := strconv.ParseInt(row[9], 10, 32)
			if err != nil {
				return nil, err
			}

			international, err := strconv.ParseBool(row[10])
			if err != nil {
				return nil, err
			}

			courses = append(courses, Course{
				AnnoAccademico:       row[0],
				Immatricolabile:      row[1],
				Codice:               int(code),
				Descrizione:          row[3],
				Url:                  row[4],
				Campus:               row[5],
				SedeDidattica:        row[6],
				Ambiti:               row[7],
				Tipologia:            row[8],
				DurataAnni:           int(years),
				Internazionale:       international,
				InternazionaleTitolo: row[11],
				InternazionaleLingua: row[12],
				Lingue:               row[13],
				Accesso:              row[14],
			})
		}
		return courses, nil
	}

	return nil, fmt.Errorf("resource is not a csv file")
}

func getPackageShowUrl(id string) string {
	return fmt.Sprintf("%s/api/3/action/package_show?id=%s", rootUnibo, id)
}

func GetPackage(id string) (*Package, error) {
	url := getPackageShowUrl(id)

	html, err := Client.Get(url)
	if err != nil {
		return nil, err
	}

	body := html.Body
	pack := Package{}

	err = json.NewDecoder(body).Decode(&pack)
	if err != nil {
		return nil, err
	}

	err = body.Close()
	if err != nil {
		return nil, err
	}

	return &pack, nil
}

type Course struct {
	AnnoAccademico       string
	Immatricolabile      string
	Codice               int
	Descrizione          string
	Url                  string
	Campus               string
	Ambiti               string
	Tipologia            string
	DurataAnni           int
	Internazionale       bool
	InternazionaleTitolo string
	InternazionaleLingua string
	Lingue               string
	Accesso              string
	SedeDidattica        string
}

type CourseWebsiteId struct {
	Tipologia string
	Id        string
}

func (c Course) RetrieveCourseWebsiteId() (CourseWebsiteId, error) {
	resp, err := Client.Get(c.Url)
	if err != nil {
		return CourseWebsiteId{}, err
	}

	buf := new(bytes.Buffer)

	// Read all body
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return CourseWebsiteId{}, err
	}

	// Close body
	err = resp.Body.Close()
	if err != nil {
		return CourseWebsiteId{}, err
	}

	// Convert body to string
	found := reg.FindStringSubmatch(buf.String())
	if found == nil {
		return CourseWebsiteId{}, fmt.Errorf("unable to find course website")
	}

	// full url -> laurea/IngegneriaInformatica
	id := found[1]

	// laurea/IngegneriaInformatica -> IngegneriaInformatica

	split := strings.Split(id, "/")
	return CourseWebsiteId{split[0], split[1]}, nil
}

func (c Course) RetrieveTimetable(anno int) ([]TimetableEvent, error) {
	id, err := c.RetrieveCourseWebsiteId()
	if err != nil {
		return nil, err
	}

	timetable, err := GetTimetable(id, anno)
	if err != nil {
		return nil, err
	}

	return timetable, nil
}

type Courses []Course

func (c Courses) Len() int {
	return len(c)
}

func (c Courses) Less(i, j int) bool {
	sort := strings.Compare(c[i].AnnoAccademico, c[j].AnnoAccademico)
	if sort == 0 {
		sort = strings.Compare(c[i].Descrizione, c[j].Descrizione)
	}

	return sort < 0
}

func (c Courses) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}