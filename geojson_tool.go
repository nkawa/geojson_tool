package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/ioutil"
	"log"
	"math"
	"os"
	"time"

	pnm "github.com/jbuchbinder/gopnm"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/planar"
	"gopkg.in/yaml.v2"
)

// geojson tool

var (
	RAND_MAX = 32767
)

var (
	file      = flag.String("file", "higashi.geojson", "Geojson File")
	yamlFname = flag.String("yaml", "out.yaml", "YAML output file name")
	jsonFname = flag.String("json", "out.json", "JSON output file name")
	pgmFile   = flag.String("pgm", "out.pgm", "PGM output file name")
	pngFile   = flag.String("png", "", "PNG output file name")
	width     = flag.Int("width", 1280, "Output PGM file width")
	//	store           = flag.Bool("store", false, "store csv data")
)

type Feature struct {
	MinLon, MinLat, MaxLon, MaxLat float64
	DLon, DLat                     float64
	Count                          int
	Scale                          float64
	PGMFile                        string
	PGMWidth, PGMHeight            int
	//	GeoJsonFC                      *geojson.FeatureCollection `json:"-"`
}

func loadGeoJson(fname string) *geojson.FeatureCollection {
	bytes, err := ioutil.ReadFile(fname)
	if err != nil {
		log.Print("Can't read file:", err)
		panic("load json")
	} else {
		fmt.Printf("Loading GeoJson file: %s\n", fname)
	}

	fc, _ := geojson.UnmarshalFeatureCollection(bytes)

	return fc
}

func setMinMax(f *Feature, lon, lat float64) {
	//	fmt.Printf("Still max:current %v %f %f\n", *f, lon, lat)
	if lon < f.MinLon {
		f.MinLon = lon
	}
	if lon > f.MaxLon {
		f.MaxLon = lon
	}
	if lat < f.MinLat {
		f.MinLat = lat
	}
	if lat > f.MaxLat {
		f.MaxLat = lat
	}
	f.Count += 1
	//	fmt.Printf("Done %v %f %f\n", *f, lon, lat)
}

func outputPGM(f *Feature, fname *string, fcs *geojson.FeatureCollection) {
	fmt.Printf("Generating Img from GeoJson\n")

	fclen := len(fcs.Features)
	f.Scale = float64(f.DLon) / float64(*width) // use Scale as a distance for each pixel.

	f.PGMWidth = int(math.Ceil(f.DLon / f.Scale))
	f.PGMHeight = int(math.Ceil(f.DLat / f.Scale))

	img := image.NewGray(image.Rect(0, 0, f.PGMWidth, f.PGMHeight))
	ticker := time.NewTicker(time.Second) // leaks www.
	for i := 0; i < fclen; i++ {
		geom := fcs.Features[i].Geometry
		if geom.GeoJSONType() == "LineString" {
		} else if geom.GeoJSONType() == "MultiLineString" {
		} else if geom.GeoJSONType() == "MultiPolygon" {
		} else if geom.GeoJSONType() == "Polygon" { // currently support only Polygon
			mp := geom.(orb.Polygon)

			for x := 0; x < f.PGMWidth; x++ {
				select {
				case <-ticker.C:
					fmt.Printf("%d%% ", int(100*x/f.PGMWidth))
				default:

				}
				for y := 0; y < f.PGMHeight; y++ {
					// polygon : mp[0] => outer line, mp[i>0] => holes
					if planar.PolygonContains(mp, orb.Point{f.MinLon + float64(x)*f.Scale, f.MaxLat - float64(y)*f.Scale}) {
						img.SetGray(x, y, color.Gray{255})
					}
				}
			}
			fmt.Print("\n")
		}
	}

	fp, err := os.Create(*fname)
	if err != nil {
		log.Fatal("Can't open file %s", *fname)
	}
	defer fp.Close()
	fmt.Printf("Writing PGM image file: %s\n", *fname)
	pnm.Encode(fp, img, pnm.PGM)

	if *pngFile != "" {
		fp2, err2 := os.Create(*pngFile)
		if err2 != nil {
			log.Fatal("Can't open file %s", *pngFile)
		}
		defer fp2.Close()
		fmt.Printf("Writing PNG image file: %s\n", *pngFile)
		png.Encode(fp2, img)
	}

}

func scanFeatures(fcs *geojson.FeatureCollection) *Feature {
	f := &Feature{
		MinLat: math.MaxFloat64,
		MinLon: math.MaxFloat64,
		MaxLat: -math.MaxFloat64,
		MaxLon: -math.MaxFloat64,
	}
	fclen := len(fcs.Features)
	//	obs := make([]*rvo.Vector2, 0, fclen)
	fmt.Printf("Checking GeoJson features: %d\n", fclen)
	for i := 0; i < fclen; i++ {
		geom := fcs.Features[i].Geometry
		//log.Printf("Geom %d: %s %v", i, geom.GeoJSONType(), geom)
		//		obs := make([]*rvo.Vector2, 0, fclen)
		if geom.GeoJSONType() == "LineString" {
			ls := geom.(orb.LineString)
			for _, pos := range ls {
				setMinMax(f, pos[0], pos[1])
			}

		} else if geom.GeoJSONType() == "MultiLineString" {
			mls := geom.(orb.MultiLineString)
			for k := 0; k < len(mls); k++ {
				//			log.Printf("%v",mls)
				coord := mls[k]
				ll := len(coord)
				for j := 0; j < ll; j++ {
					setMinMax(f, coord[j][0], coord[j][1])
				}
			}
		} else if geom.GeoJSONType() == "MultiPolygon" {
			mp := geom.(orb.MultiPolygon)
			ls := mp[0][0]
			ll := len(ls) // linestring
			fmt.Printf("Multi:obs len %d:%d\n", i, ll)
			for j := 0; j < ll; j++ {
				setMinMax(f, ls[j][0], ls[j][1])
			}
		} else if geom.GeoJSONType() == "Polygon" {
			mp := geom.(orb.Polygon)
			ls := mp[0]
			ll := len(ls) // linestring
			fmt.Printf("Scanning Polygons len %d:%d\n", i, ll)
			for j := 0; j < ll; j++ {
				setMinMax(f, ls[j][0], ls[j][1])
			}
		}

	}
	f.DLat = f.MaxLat - f.MinLat
	f.DLon = f.MaxLon - f.MinLon
	return f
}

func main() {
	flag.Parse()
	var feature = &Feature{}
	var fcs *geojson.FeatureCollection
	if *file != "" { // load geojson
		fcs = loadGeoJson(*file)
		feature = scanFeatures(fcs)
		fmt.Printf("Loaded GeoJson count %d\n", feature.Count)
		fmt.Printf("Min  %f, %f\n", feature.MinLon, feature.MinLat)
		fmt.Printf("Max  %f, %f\n", feature.MaxLon, feature.MaxLat)
		fmt.Printf("Delta %f, %f\n", feature.DLon, feature.DLat)
	}
	if *pgmFile != "" { // we need to output!
		if feature.DLon == 0 {
			log.Fatal("Zero width!")
		}
		feature.Scale = feature.DLon / float64(*width)
		outputPGM(feature, pgmFile, fcs)
		feature.PGMFile = *pgmFile
	}

	if *jsonFname != "" {
		file, err := os.Create(*jsonFname)
		if err != nil {
			log.Fatal("Can't open ", *jsonFname)
		}
		defer file.Close()

		//		b, _ := json.Marshal(&feature)
		b, _ := json.MarshalIndent(&feature, "", "   ")
		file.Write(b)
	}
	if *yamlFname != "" {
		file, err := os.Create(*yamlFname)
		if err != nil {
			log.Fatal("Can't open ", *yamlFname)
		}
		defer file.Close()
		b, _ := yaml.Marshal(&feature)
		file.Write(b)
	}

}
