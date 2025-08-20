# Conversions
Converts a flat json into data conforming to the OSC-EM schema. 

## Usage
The converter can take any **flat json** to convert to OSC-EM, provided a mapping table in form of `.csv` .
The csv needs to follow a similar approach to the [default one](csv/ls_conversions.csv) for life sciences, albeit at a reduced complexity, like the ones for [materials science](#materials-science-ms_conversions_emdcsv-ms_conversions_przcsv).
It requires the following columns:
- **oscem**: The OSC-EM field to map to. Fields are `.` separated for nesting. The `[N]` notation is used for arrays.
- **fromformat**: What the key is called in the input format json.
- **optionals**: If there are any optional namings that might map to the same field, at an increased priority if present.
- **units**: The unit of any given field, if applicable.
- **crunch**: The conversion factor to arrive at your desired output unit, based on the value in the input json. 
- **type**: The type of the field. Allowed values are: Int, String, Float64, Bool.

When using the Converter as a standalone tool, you can compile it using the `cmd/convert_cli/` path, then:

```sh
go build main.go
```

It accepts the following inputs:
- `-i`: input json
- `-o`: output filename (optional, will take directory name if none provided)
- `-map`: path to the mapping file described above 
- `-cs`: allows you to provide the cs (spherical aberration) value for your instrument (optional)
- `-gain_flip_rotate`: allows to provide instructions on gainreference flipping if needed (optional)


If you want to use it inside of another go application you can also just import it as a module using:
```sh
import github.com/oscem/Converter
```

## Mapping Tables

All mapping tables can be found in the `csv/` directory.
When modifying any table, it is crucial to save it in UTF-8 compatible format - otherwise some of the units will fail.

The nesting of the OSC-EM schema is described as `.` separated in the first column of the table.
Arrays use the `[N]` notation.

### Life sciences: `ls_conversions.csv`

This table is the heart of the actual conversion from instrument metadata output to metadata conforming to the OSC-EM schema.
Currently, it maps OSC-EM fields to the outputs of EPU (xml), SerialEM (mdoc) and Tomo5 (both mdocs and xmls).
All file formats are included into this one mapping table.
Additionally, it assigns units to fields and crunches conversions to reach the target unit from the original output.

> **Note:**
> Units need updates if the instrument softwares change the way they output metadata - none of them explicity specify the units of their fields.

This also means that in order to use the table on a new or extended schema you also need to update the mapping table to match the new schema (additions only - unused mappings are irrelevant but might be useful for a different schema).

### Materials science: `ms_conversions_emd.csv`, `ms_conversions_prz.csv`

To reduce complexity, a separate file was created for each materials science metadata format.
Currently, the `.emd` and `.prz` metadata file formats are covered.

These mapping tables follow the structure that was described above.
They make extensive use of the `[N]` notation for arrays, especially to accomodate the usage of multiple detectors per acquisition.
It can be used in the following columns:
- **oscem**: Here `[N]` signifies that this OSC-EM field in an array. The elements of this array will be objects whose keys can be defined using the `.` separator as usual.
- **fromformat**: When `[N]` is used here it operates as a placeholder, signifying that more than one metadata keys that follow a similar name convension might map to this OSC-EM field.
For example, _Detectors.Detector-[N].DetectorName_ means that there could be multiple detectors following this naming pattern, where `[N]` could be replaced by anything, i.e. _Detectors.Detector-1.DetectorName_, _Detectors.Detector-ABC.DetectorName_, etc.
All of those will be collected and form separate objects that will be added as elements to the corresponding array.
In the case that the metadata keys do not follow a specific naming pattern that can be covered by the `[N]` notation, it is also possible to map them to the OSC-EM field inividually, by using a `;` separator within each _fromformat_ cell value.

### Mapping to PDB: `pdb_conversions.csv`

Lastly, this table maps (parts of) the OSC-EM schema to the PDB/EMDB mmcif dictionary.
This is required for the [oscem-to-mmcif-converter](https://github.com/osc-em/converter-OSCEM-to-mmCIF).
