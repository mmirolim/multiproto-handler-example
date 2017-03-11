package main

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

var (
	// verify interface implementation
	_ csvUnmarshaler = &msg{}
	_ csvMarshaler   = &reply{}
	// data exchange protocols
	protos = map[string]struct {
		From func(data []byte, v interface{}) error
		To   func(v interface{}) ([]byte, error)
	}{
		"json": {json.Unmarshal, json.Marshal},
		"xml":  {xml.Unmarshal, xml.Marshal},
		"csv":  {fromCSV, toCSV},
		// TODO add protobuf, some custom formats
	}
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/process", SetMethod(http.MethodPost, SetProto(ProcessMsgHandlerFunc)))
	log.Fatal(http.ListenAndServe(":8081", mux))
}

// Business logic start
func processMsg(m msg) reply {
	// do something with msg
	// validate
	if validate(m) {
		return reply{"OK", "got msg::" + m.Text}
	} else {
		return reply{"NOTOK", "did not get it"}
	}
}

func validate(m msg) bool {
	if m.Name != "" && m.Text != "" {
		return true
	} else {
		return false
	}
}

type msg struct {
	Name string `xml:"name"`
	Text string `xml:"text"`
}

type reply struct {
	Status string
	Text   string
}

// Business logic end

// Handling http request/response
func ProcessMsgHandlerFunc(protoFrom, protoTo string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var m msg
		err := Decode(r.Body, protos[protoFrom].From, &m)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		response, err := protos[protoTo].To(processMsg(m))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Write(response)
	}
}

// Handling data exchange formats start
func (m *msg) csvUnmarshal(data []byte) error {
	strs := strings.Split(string(data), ",")
	m.Name = strs[0]
	m.Text = strs[1]
	return nil
}

func (r reply) csvMarshal() ([]byte, error) {
	return []byte(strings.Join([]string{r.Status, r.Text}, ",")), nil
}

// Handling data exchange formats end

// Boilerplates start
func SetMethod(method string, f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == method {
			f(w, r)
		} else {
			http.Error(w, "wrong http method", http.StatusBadRequest)
		}
	}
}

func SetProto(f func(string, string) http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// protoFrom get from content type
		// protoTo get from header
		conttype := strings.Split(r.Header.Get("Content-type"), "/")
		if len(conttype) == 2 {
			if _, ok := protos[conttype[1]]; ok {
				protoFrom := conttype[1]
				restype := strings.Split(r.Header.Get("Response-type"), "/")
				// default response proto
				protoTo := protoFrom
				if len(restype) == 2 {
					if _, ok := protos[restype[1]]; ok {
						protoTo = restype[1]
					}
				}

				f(protoFrom, protoTo)(w, r)
				return
			}
		}

		http.Error(w, "not supported proto", http.StatusBadRequest)

	}
}

func Decode(
	r io.ReadCloser,
	proto func([]byte, interface{}) error,
	strct interface{},
) error {
	if r == nil {
		return errors.New("nothing to do")
	}

	defer r.Close()
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return errors.New("read req body err")
	}

	return proto(data, strct)
}

type csvMarshaler interface {
	csvMarshal() ([]byte, error)
}

type csvUnmarshaler interface {
	csvUnmarshal([]byte) error
}

// custom csv marshal, unmarshal
func fromCSV(data []byte, v interface{}) error {
	if csver, ok := v.(csvUnmarshaler); !ok {
		return errors.New("Csver interface not impled")
	} else {
		return csver.csvUnmarshal(data)
	}
}

func toCSV(v interface{}) ([]byte, error) {
	if csver, ok := v.(csvMarshaler); !ok {
		return nil, errors.New("Csver interface not impled")
	} else {
		return csver.csvMarshal()
	}
}

// Boilerplates ends
