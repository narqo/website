// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/microcosm-cc/bluemonday"
	"gopkg.in/yaml.v2"
)

const (
	// eventLimit is the maximum number of events that will be output.
	eventLimit = 15
	// groupsSummaryPath is an API endpoint that returns global Go groups.
	// Fetching from this API path allows to sort groups by next upcoming event.
	groupsSummaryPath = "/pro/go/es_groups_summary?location=global&order=next_event&desc=false"
	// eventsHeader is a header comment for the output content.
	eventsHeader = `# DO NOT EDIT: Autogenerated from cmd/events.
# To update, run:
#    go run github.com/godevsite/go.dev/cmd/events > data/events.yaml`
)

func main() {
	c := &meetupAPI{
		baseURL: "https://api.meetup.com",
	}
	ue, err := getUpcomingEvents(c)
	if err != nil {
		log.Fatal(err)
	}
	printYAML(ue)
}

type client interface {
	getGroupsSummary() (*GroupsSummary, error)
	getGroup(urlName string) (*Group, error)
}

// getUpcomingEvents returns upcoming events globally.
func getUpcomingEvents(c client) (*UpcomingEvents, error) {
	summary, err := c.getGroupsSummary()
	if err != nil {
		return nil, err
	}
	p := bluemonday.NewPolicy()
	p.AllowStandardURLs()
	p.AllowAttrs("href").OnElements("a")
	p.AllowElements("br")
	// Work around messy newlines in content.
	r := strings.NewReplacer("\n", "<br/>\n", "&lt;br&gt;", "<br/>\n")
	var events []EventData
	for _, chapter := range summary.Chapters {
		if len(events) >= eventLimit {
			break
		}
		group, err := c.getGroup(chapter.URLName)
		if err != nil || group.NextEvent == nil {
			continue
		}
		tz, err := time.LoadLocation(group.Timezone)
		if err != nil {
			tz = time.UTC
		}
		// group.NextEvent.Time is in milliseconds since UTC epoch.
		nextEventTime := time.Unix(group.NextEvent.Time/1000, 0).In(tz)
		events = append(events, EventData{
			City:              chapter.City,
			Country:           chapter.Country,
			Description:       r.Replace(p.Sanitize(chapter.Description)), // Event descriptions are often blank, use Group description.
			ID:                group.NextEvent.ID,
			LocalDate:         nextEventTime.Format("Jan 2, 2006"),
			LocalTime:         nextEventTime.Format(time.RFC3339),
			LocalizedCountry:  group.LocalizedCountryName,
			LocalizedLocation: group.LocalizedLocation,
			Name:              group.NextEvent.Name,
			State:             chapter.State,
			ThumbnailURL:      chapter.GroupPhoto.ThumbLink,
			URL:               "https://www.meetup.com/" + path.Join(chapter.URLName, "events", group.NextEvent.ID),
		})
	}
	return &UpcomingEvents{All: events}, nil
}

type meetupAPI struct {
	baseURL string
}

func (c *meetupAPI) getGroupsSummary() (*GroupsSummary, error) {
	resp, err := http.Get(c.baseURL + groupsSummaryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get events from %q: %v", groupsSummaryPath, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get events from %q: %v", groupsSummaryPath, resp.Status)
	}
	var summary *GroupsSummary
	d := json.NewDecoder(resp.Body)
	if err := d.Decode(&summary); err != nil {
		return summary, fmt.Errorf("failed to decode events from %q: %w", groupsSummaryPath, err)
	}
	return summary, nil
}

// getGroup fetches group details, which are useful for getting details of the next upcoming event, and timezones.
func (c *meetupAPI) getGroup(urlName string) (*Group, error) {
	u := c.baseURL + "/" + urlName
	resp, err := http.Get(u)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch group details from %q: %w", u, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch group details from %q: %v", u, resp.Status)
	}

	var group Group
	d := json.NewDecoder(resp.Body)
	if err := d.Decode(&group); err != nil {
		return nil, fmt.Errorf("failed to decode group from %q: %w", u, err)
	}
	return &group, nil
}

func printYAML(v interface{}) {
	fmt.Println(eventsHeader)
	e := yaml.NewEncoder(os.Stdout)
	defer e.Close()
	if err := e.Encode(v); err != nil {
		log.Fatalf("failed to encode event yaml: %v", err)
	}
}

// UpcomingEvents is a list of EventData written out to YAML for rendering in Hugo.
type UpcomingEvents struct {
	All []EventData
}

// EventData is the structure written out to YAML for rendering in Hugo.
type EventData struct {
	City              string
	Country           string
	Description       string
	ID                string
	LocalDate         string `yaml:"local_date"`
	LocalTime         string `yaml:"local_time"`
	LocalizedCountry  string
	LocalizedLocation string
	Name              string
	State             string
	ThumbnailURL      string
	URL               string
}

// GroupsSummary is the structure returned from /pro/go/es_groups_summary.
type GroupsSummary struct {
	Chapters []*Chapter
}

type Event struct {
	Created       int    `json:"created"`
	Description   string `json:"description"`
	Duration      int    `json:"duration"`
	Fee           *Fee   `json:"fee"`
	Group         *Group `json:"group"`
	LocalDate     string `json:"local_date"`
	LocalTime     string `json:"local_time"`
	ID            string `json:"id"`
	Link          string `json:"link"`
	Name          string `json:"name"`
	RSVPLimit     int    `json:"rsvp_limit"`
	Status        string `json:"status"`
	Time          int64  `json:"time"`
	UTCOffset     int    `json:"utc_offset"`
	Updated       int    `json:"updated"`
	Venue         *Venue `json:"venue"`
	WaitlistCount int    `json:"waitlist_count"`
	YesRSVPCount  int    `json:"yes_rsvp_count"`
}

type Venue struct {
	Address1             string  `json:"address_1"`
	Address2             string  `json:"address_2"`
	Address3             string  `json:"address_3"`
	City                 string  `json:"city"`
	Country              string  `json:"country"`
	ID                   int     `json:"id"`
	Lat                  float64 `json:"lat"`
	LocalizedCountryName string  `json:"localized_country_name"`
	Lon                  float64 `json:"lon"`
	Name                 string  `json:"name"`
	Repinned             bool    `json:"repinned"`
	State                string  `json:"state"`
	Zip                  string  `json:"zip"`
}

type Group struct {
	Country              string  `json:"country"`
	Created              int     `json:"created"`
	Description          string  `json:"description"`
	ID                   int     `json:"id"`
	JoinMode             string  `json:"join_mode"`
	Lat                  float64 `json:"lat"`
	LocalizedLocation    string  `json:"localized_location"`
	LocalizedCountryName string  `json:"localized_country_name"`
	Lon                  float64 `json:"lon"`
	Name                 string  `json:"name"`
	NextEvent            *Event  `json:"next_event"`
	Region               string  `json:"region"`
	Timezone             string  `json:"timezone"`
	URLName              string  `json:"urlname"`
	Who                  string  `json:"who"`
}

type Fee struct {
	Accepts     string  `json:"accepts"`
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	Description string  `json:"description"`
	Label       string  `json:"label"`
	Required    bool    `json:"required"`
}

type Chapter struct {
	AverageAge     float64        `json:"average_age"`
	Category       []Category     `json:"category"`
	City           string         `json:"city"`
	Country        string         `json:"country"`
	Description    string         `json:"description"`
	FoundedDate    int64          `json:"founded_date"`
	GenderFemale   float64        `json:"gender_female"`
	GenderMale     float64        `json:"gender_male"`
	GenderOther    float64        `json:"gender_other"`
	GenderUnknown  float64        `json:"gender_unknown"`
	GroupPhoto     GroupPhoto     `json:"group_photo"`
	ID             int            `json:"id"`
	LastEvent      int64          `json:"last_event"`
	Lat            float64        `json:"lat"`
	Lon            float64        `json:"lon"`
	MemberCount    int            `json:"member_count"`
	Name           string         `json:"name"`
	NextEvent      int64          `json:"next_event"`
	OrganizerPhoto OrganizerPhoto `json:"organizer_photo"`
	Organizers     []Organizer    `json:"organizers"`
	PastEvents     int            `json:"past_events"`
	PastRSVPs      int            `json:"past_rsvps"`
	ProJoinDate    int64          `json:"pro_join_date"`
	RSVPsPerEvent  float64        `json:"rsvps_per_event"`
	RepeatRSVPers  int            `json:"repeat_rsvpers"`
	State          string         `json:"state"`
	Status         string         `json:"status"`
	Topics         []Topic        `json:"topics"`
	URLName        string         `json:"urlname"`
	UpcomingEvents int            `json:"upcoming_events"`
}

type Topic struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	URLkey string `json:"urlkey"`
	Lang   string `json:"lang"`
}

type Category struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Shortname string `json:"shortname"`
	SortName  string `json:"sort_name"`
}

type Organizer struct {
	Name       string `json:"name"`
	MemberID   int    `json:"member_id"`
	Permission string `json:"permission"`
}

type OrganizerPhoto struct {
	BaseURL     string `json:"base_url"`
	HighresLink string `json:"highres_link"`
	ID          int    `json:"id"`
	PhotoLink   string `json:"photo_link"`
	ThumbLink   string `json:"thumb_link"`
	Type        string `json:"type"`
}

type GroupPhoto struct {
	BaseURL     string `json:"base_url"`
	HighresLink string `json:"highres_link"`
	ID          int    `json:"id"`
	PhotoLink   string `json:"photo_link"`
	ThumbLink   string `json:"thumb_link"`
	Type        string `json:"type"`
}
