package bagins

/*

"Oft in lies truth is hidden."

- Glorfindel

*/

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/APTrust/bagins/bagutil"
	"hash"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

/*
 Manifest represents information about a BagIt manifest file.  As of BagIt spec
 0.97 this means only manifest-<algo>.txt and tagmanifest-<algo>.txt files.

 For more information see:
   manifest: http://tools.ietf.org/html/draft-kunze-bagit-09#section-2.1.3
   tagmanifest: http://tools.ietf.org/html/draft-kunze-bagit-09#section-2.2.1
*/
type Manifest struct {
	name     string            // Path to the
	Data     map[string]string // Map of filepath key and checksum value for that file
	hashFunc func() hash.Hash
}

// Returns a pointer to a new manifest or returns an error if improperly named.
func NewManifest(pth string, hashName string) (*Manifest, error) {
	if _, err := os.Stat(filepath.Dir(pth)); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("Unable to create manifest. Path does not exist: %s", pth)
		} else {
			return nil, fmt.Errorf("Unexpected error creating manifest: %s", err)
		}
	}
	m := new(Manifest)
	hashFunc, err := bagutil.LookupHash(hashName)
	if err != nil {
		return nil, err
	}
	m.hashFunc = hashFunc
	m.Data = make(map[string]string)

	// TODO: Fix this!
	// First option is required if user passed a manifest file name into Bag.ReadBag().
	// Second option is required if no manifest file name was passed in to Bag.ReadBag().
	if filepath.Dir(pth) != "" && strings.Contains(pth, "manifest-") {
		m.name = pth
	} else {
		m.name = filepath.Join(pth, "manifest-"+strings.ToLower(hashName)+".txt")
	}

	return m, nil
}

/*
  Opens a manifest file, parses attemps to parse the hashtype from the filename, parses
  the file contents and returns a pointer to a Manifest.  Error slice may comprise multiple
  parsing errors when attempting to read data for fault tolerance.
*/
func ReadManifest(name string) (*Manifest, []error) {
	var errs []error

	hashName, err := parseAlgoName(name)
	if err != nil {
		return nil, append(errs, err)
	}

	file, err := os.Open(name)
	if err != nil {
		return nil, append(errs, err)
	}

	data, e := parseManifestData(file)
	if e != nil {
		errs = append(errs, e...)
	}

	m, err := NewManifest(name, hashName)
	if err != nil {
		return nil, append(errs, err)
	}
	m.Data = data

	return m, errs

}

/*
  Calculates a checksum for files listed in the manifest and compares it to the value
  stored in manifest file.  Returns an error for each file that fails the fixity check.
*/
func (m *Manifest) RunChecksums() []error {
	var invalidSums []error

	for key, sum := range m.Data {
		pth := filepath.Join(filepath.Dir(m.name), key)
		fileChecksum, err := bagutil.FileChecksum(pth, m.hashFunc())
		if sum != fileChecksum {
			invalidSums = append(invalidSums, fmt.Errorf("File checksum %s is not valid for %s:%s", sum, key, fileChecksum))
		}
		if err != nil {
			invalidSums = append(invalidSums, err)
		}
	}

	return invalidSums
}

// Writes key value pairs to a manifest file.
func (m *Manifest) Create() error {
	if m.Name() == "" {
		return errors.New("Manifest must have values for basename and algo set to create a file.")
	}
	// Create directory if needed.
	basepath := filepath.Dir(m.name)

	if err := os.MkdirAll(basepath, 0777); err != nil {
		return err
	}

	// Create the tagfile.
	fileOut, err := os.Create(m.name)
	if err != nil {
		return err
	}
	defer fileOut.Close()

	// Write fields and data to the file.
	m.Write(fileOut)
	/*
		for fName, ckSum := range m.Data {
			_, err := fmt.Fprintln(fileOut, ckSum, fName)
			if err != nil {
				return errors.New("Error writing line to manifest: " + err.Error())
			}
		}
	*/
	return nil
}

// Returns the contents of the manifest in the form of a string.
// Useful if you don't want to write directly to disk.
func (m *Manifest) ToString() string {
	buf := bytes.NewBuffer(nil)
	m.Write(buf)
	return buf.String()
}

func (m *Manifest) Write(w io.Writer) error {
	keys := []string{}
	for fName, _ := range m.Data {
		keys = append(keys, fName)
	}
	sort.Strings(keys)
	for _, key := range keys {
		io.WriteString(w, m.Data[key])
		io.WriteString(w, " ")
		io.WriteString(w, key)
		io.WriteString(w, "\n")
	}
	return nil
}

// Returns a sting of the filename for this manifest file based on Path, BaseName and Algo
func (m *Manifest) Name() string {
	return filepath.Clean(m.name)
}

// Tries to parse the algorithm name from a manifest filename.  Returns
// an error if unable to do so.
func parseAlgoName(name string) (string, error) {
	filename := filepath.Base(name)
	re, err := regexp.Compile(`(^.*\-)(.*)(\.txt$)`)
	if err != nil {
		return "", err
	}
	matches := re.FindStringSubmatch(filename)
	if len(matches) < 2 {
		return "", errors.New("Unable to determine algorithm from filename!")
	}
	algo := matches[2]
	return algo, nil
}

func ParseManifestData(file io.Reader) (map[string]string, []error) {
	return parseManifestData(file)
}

// Reads the contents of file and parses checksum and file information in manifest format as
// per the bagit specification.
func parseManifestData(file io.Reader) (map[string]string, []error) {
	var errs []error
	// See regexp examples at http://play.golang.org/p/_msLJ-lBEu
	// Regex matches these reqs from the bagit spec: "One or
	// more linear whitespace characters (spaces or tabs) MUST separate
	// CHECKSUM from FILENAME." as specified here:
	// http://tools.ietf.org/html/draft-kunze-bagit-10#section-2.1.3
	re := regexp.MustCompile(`^(\S*)\s*(.*)`)

	scanner := bufio.NewScanner(file)
	values := make(map[string]string)

	for scanner.Scan() {
		line := scanner.Text()
		if re.MatchString(line) {
			data := re.FindStringSubmatch(line)
			values[data[2]] = data[1]
		} else {
			errs = append(errs, fmt.Errorf("Unable to parse data from line: %s", line))
		}
	}

	return values, errs
}
