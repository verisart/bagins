/*
Methods for managing tag files inside of bags.  These represent the standard
tag files that are part of every Bag spec or optional tag files which can
be used liberally throughout the bag itself.

For more information on Bag tagfiles see
http://tools.ietf.org/html/draft-kunze-bagit-09#section-2.3
*/
package bagins

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
)

type TagFile struct {
	name string            // Filepath for tag file.
	Data map[string]string // key value pairs of data for the tagfile.
}

func NewTagFile(name string) (tf *TagFile, err error) {
	_, err = os.Stat(path.Dir(name))
	re, _ := regexp.Compile(`.*\.txt`)
	if !re.MatchString(path.Base(name)) {
		err = errors.New(fmt.Sprint("Tagfiles must end in .txt and contain at least 1 letter.  Provided: ", path.Base(name)))
	}
	tf = new(TagFile)
	tf.name = path.Clean(name)
	tf.Data = make(map[string]string)
	return tf, err
}

// Writes key value pairs to a tag file.
func (tf *TagFile) Create() {
	// Create directory if needed.
	if err := os.MkdirAll(path.Dir(tf.name), 0777); err != nil {
		panic("Error creating tagfile directory: " + err.Error())
	}

	// Create the tagfile.
	fileOut, err := os.Create(tf.name)
	if err != nil {
		panic("Error creating tagfile: " + err.Error())
	}
	defer fileOut.Close()

	// Write fields and data to the file.
	for key, data := range tf.Data {
		_, err := fmt.Fprintln(fileOut, FormatField(key, data))
		if err != nil {
			panic("Error writing line to tagfile: " + err.Error())
		}
	}
}

/*
Takes a tag field key and data and wraps lines at 79 with indented spaces as
per recommendation in spec.
*/
func FormatField(key string, data string) string {
	delimeter := "\n   "
	var buff bytes.Buffer

	// Initiate it by writing the proper key.
	writeLen, err := buff.WriteString(fmt.Sprintf("%s: ", key))
	if err != nil {
		panic("Error intiating field: " + err.Error())
	}
	splitCounter := writeLen

	words := strings.Split(data, " ")

	for word := range words {
		if splitCounter+len(words[word]) > 79 {
			splitCounter, err = buff.WriteString(delimeter)
			if err != nil {
				panic("Error inserting newline in field: " + err.Error())
			}
		}
		writeLen, err = buff.WriteString(strings.Join([]string{" ", words[word]}, ""))
		if err != nil {
			panic("Error writing data to field: " + err.Error())
		}
		splitCounter += writeLen

	}
	return buff.String()
}
