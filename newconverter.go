package conversion

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/osc-em/Converter/basetypes"
)

//go:embed conversions.csv
var embedded embed.FS

type FieldSpec struct {
	Path []string
	Type string
}

func Convert(jsonin []byte, contentFlag string, p1Flag string, p2Flag string, oFlag string) ([]byte, error) {
	var rows []csvextract
	if contentFlag != "" {
		var err error
		rows, err = loadMappingCSV(contentFlag) // custom
		if err != nil {
			log.Fatal(err)
			return nil, err
		}
	} else {
		var err error
		rows, err = readCSVFile(embedded) // default
		if err != nil {
			log.Fatal(err)
			return nil, err
		}
	}

	var values map[string]string
	_ = json.Unmarshal(jsonin, &values)

	out, err := convertToHierarchicalJSON(rows, values)
	if err != nil {
		log.Fatal(err)
	}
	// placeholder for adding from flags later
	cs := p1Flag
	gainref_flip_rotate := p2Flag

	casted := castToBaseType(cs, "float64", "mm")
	casted2 := castToBaseType(gainref_flip_rotate, "string", "")

	insertNested(out, []string{"instrument", "cs"}, casted)
	insertNested(out, []string{"acquisition", "gainref_flip_rotate"}, casted2)

	// this allows us to obtain nil values for types where Go usually doesnt allow them e.g. int
	cleaned := CleanMap(out)

	pretty, _ := json.MarshalIndent(cleaned, "", "  ")
	if oFlag == "" {
		cwd, _ := os.Getwd()
		cut := strings.Split(cwd, string(os.PathSeparator))
		name := cut[len(cut)-1] + ".json"
		os.WriteFile(name, pretty, 0644)
		fmt.Println()
		fmt.Println("Extracted data was written to: ", name)

	} else {
		twd := oFlag
		if !strings.Contains(twd, ".json") {
			var conc []string
			conc = append(conc, twd, "json")
			twd = strings.Join(conc, ".")
		}
		os.WriteFile(twd, pretty, 0644)
		fmt.Println()
		fmt.Printf("Extracted data was written to: %s", twd)
	}

	return pretty, nil
}

func CleanMap(data interface{}) interface{} {
	switch v := data.(type) {

	case map[string]interface{}:
		cleanedMap := make(map[string]interface{})
		for key, value := range v {
			cleanedValue := CleanMap(value)
			if cleanedValue != nil {
				cleanedMap[key] = cleanedValue
			}
		}
		if len(cleanedMap) == 0 {
			return nil
		}
		return cleanedMap

	case []interface{}:
		var cleanedSlice []interface{}
		for _, elem := range v {
			cleanedElem := CleanMap(elem)
			if cleanedElem != nil {
				cleanedSlice = append(cleanedSlice, cleanedElem)
			}
		}
		if len(cleanedSlice) == 0 {
			return nil
		}
		return cleanedSlice

	case basetypes.Int:
		if v.HasSet {
			return v
		}
		return nil
	case basetypes.Float64:
		if v.HasSet {
			return v
		}
		return nil
	case basetypes.Bool:
		if v.HasSet {
			return v
		}
		return nil
	case basetypes.String:
		if v.HasSet {
			return v
		}
		return nil

	default:
		// Primitive types that are set directly
		if v == nil {
			return nil
		}
		return v
	}
}

// Convert string to appropriate type
func castToBaseType(value string, t string, unit string) interface{} {
	switch strings.ToLower(t) {
	case "int":
		var val int64
		fmt.Sscanf(value, "%d", &val)
		var out basetypes.Int
		out.Set(val, unit) // sets .HasSet = true
		return out

	case "float", "float64":
		var val float64
		fmt.Sscanf(value, "%f", &val)
		var out basetypes.Float64
		out.Set(val, unit) // sets .HasSet = true
		return out

	case "bool":
		var out basetypes.Bool
		out.Set(strings.ToLower(value) == "true") // sets .HasSet = true
		return out

	case "string":
		var out basetypes.String
		out.Set(value) // sets .HasSet = true
		return out

	default:
		return nil
	}
}
