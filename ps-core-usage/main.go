package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

var ExpectedColumns int = 59
var PlayerIndex int = 0
var TypeIndex int = 1
var LeadIndex int = 2
var PokemonsStart int = 4
var PokemonsColumns int = 9

type StatsFilter struct {
	For     TeamFilter `json:"for"`
	Against TeamFilter `json:"against"`
}

type TeamFilter struct {
	Player   []string   `json:"player"`   // Or between names
	Pokemons [][]string `json:"pokemons"` // Or of ands
	Type     []string   `json:"type"`     // Or between types
	Lead     []string   `json:"lead"`
	Dynamax  []string   `json:"dynamax"`
}

type Output struct {
	Size    int  `json:"size"`
	Lead    bool `json:"lead"`
	Dynamax bool `json:"dynamax"`
}

func main() {
	args := os.Args
	if len(args) != 4 {
		fmt.Println("go run main.go file output teamFilter")
		return
	}

	file := args[1]

	b, err := ioutil.ReadFile(file)
	if err != nil {
		fmt.Println(err)
		return
	}

	fileContent := string(b)
	lines := strings.Split(fileContent, "\n")

	var output Output
	err = json.Unmarshal([]byte(args[2]), &output)
	if err != nil {
		fmt.Println(err)
		return
	}

	var filter StatsFilter
	err = json.Unmarshal([]byte(args[3]), &filter)
	if err != nil {
		fmt.Println(err)
		return
	}

	lines = filter.filterLines(lines)

	PrintComboUsage(output, lines)
}

func PrintComboUsage(output Output, lines []string) {
	if output.Lead {
		getSpecific(lines, LeadIndex)
		return
	}

	combos := allCombo(output.Size)
	cores := map[string]int{}
	scores := map[string]int{}
	kills := map[string]int{}
	deaths := map[string]int{}
	for _, line := range lines {
		mons := strings.Split(line, ";")
		if len(mons) != ExpectedColumns {
			continue
		}

		mons = mons[PokemonsStart:] // Cut player and lead and type
		for _, combo := range combos {
			comboKills := 0
			comboDeaths := 0
			keys := make([]string, output.Size)
			i := 0
			for j, index := range combo {
				if mons[j*PokemonsColumns] == "" {
					continue
				}

				if index == 1 {
					keys[i] = mons[j*PokemonsColumns]
					pokeKills, _ := strconv.Atoi(mons[j*PokemonsColumns+6]) // Beware index /!\
					comboKills += pokeKills
					pokeDeaths, _ := strconv.Atoi(mons[j*PokemonsColumns+7]) // Beware index /!\
					comboDeaths += pokeDeaths
					i++
				}
			}
			if i != output.Size {
				continue
			}
			cores[strings.Join(keys, ";")]++
			kills[strings.Join(keys, ";")] += comboKills
			deaths[strings.Join(keys, ";")] += comboDeaths
			if mons[len(mons)-1] == "W" {
				scores[strings.Join(keys, ";")]++
			}
		}
	}

	threshold := 0
	if output.Size == 2 || output.Size == 5 {
		threshold = 2
	}

	if output.Size == 3 || output.Size == 4 {
		threshold = 3
	}

	for name, value := range cores {
		if value >= threshold {
			fmt.Println(name + ";" +
				strconv.Itoa(value) + ";" +
				strconv.Itoa(scores[name]) + ";" +
				strconv.Itoa(kills[name]) + ";" +
				strconv.Itoa(deaths[name]))
		}
	}
}

func (f StatsFilter) filterLines(lines []string) []string {
	res := make([]string, len(lines))
	j := 0
	i := 0
	for i < len(lines)-1 {
		team1 := strings.Split(lines[i], ";")
		team2 := strings.Split(lines[i+1], ";")
		if f.For.matchTeam(team1) && f.Against.matchTeam(team2) {
			res[j] = lines[i]
			j++
		}
		if f.For.matchTeam(team2) && f.Against.matchTeam(team1) {
			res[j] = lines[i+1]
			j++
		}

		i += 2
	}

	return res[:j]
}

func (f *TeamFilter) matchTeam(team []string) bool {
	if len(f.Player) != 0 && !stringInSliceInsensitive(team[PlayerIndex], f.Player) {
		return false
	}

	if len(f.Lead) != 0 && !stringInSlice(team[LeadIndex], f.Lead) {
		return false
	}

	if len(f.Type) != 0 && !stringInSlice(team[TypeIndex], f.Type) {
		return false
	}

	if len(f.Pokemons) != 0 && !f.pokemonsMatch(team[PokemonsStart:]) {
		return false
	}

	return true
}

func (f *TeamFilter) pokemonsMatch(team []string) bool {
	for _, andPoke := range f.Pokemons {
		if include(team, andPoke) {
			return true
		}
	}

	return false
}

func getSpecific(lines []string, index int) {
	cores := map[string]int{}
	scores := map[string]int{}

	for _, line := range lines {
		mons := strings.Split(line, ";")
		if len(mons) != ExpectedColumns {
			continue
		}

		cores[mons[index]]++
		if mons[len(mons)-1] == "W" {
			scores[mons[index]]++
		}
	}

	threshold := 0
	for name, value := range cores {
		if value > threshold {
			fmt.Println(name + ";" + strconv.Itoa(value) + ";" + strconv.Itoa(scores[name]))
		}
	}
}

func stringInSlice(p1 string, ps []string) bool {
	for _, p2 := range ps {
		if p1 == p2 {
			return true
		}
	}

	return false
}

func stringInSliceInsensitive(p1 string, ps []string) bool {
	for _, p2 := range ps {
		if strings.ToLower(p1) == strings.ToLower(p2) {
			return true
		}
	}

	return false
}

func allCombo(i int) [][]int {
	switch i {
	case 1:
		return [][]int{
			{0, 0, 0, 0, 0, 1},
			{0, 0, 0, 0, 1, 0},
			{0, 0, 0, 1, 0, 0},
			{0, 0, 1, 0, 0, 0},
			{0, 1, 0, 0, 0, 0},
			{1, 0, 0, 0, 0, 0},
		}
	case 2:
		return [][]int{
			{0, 0, 0, 0, 1, 1},
			{0, 0, 0, 1, 0, 1},
			{0, 0, 1, 0, 0, 1},
			{0, 1, 0, 0, 0, 1},
			{1, 0, 0, 0, 0, 1},
			{0, 0, 0, 1, 1, 0},
			{0, 0, 1, 0, 1, 0},
			{0, 1, 0, 0, 1, 0},
			{1, 0, 0, 0, 1, 0},
			{0, 0, 1, 1, 0, 0},
			{0, 1, 0, 1, 0, 0},
			{1, 0, 0, 1, 0, 0},
			{0, 1, 1, 0, 0, 0},
			{1, 0, 1, 0, 0, 0},
			{1, 1, 0, 0, 0, 0},
		}
	case 3:
		return [][]int{
			{0, 0, 0, 1, 1, 1},
			{0, 0, 1, 0, 1, 1},
			{0, 1, 0, 0, 1, 1},
			{1, 0, 0, 0, 1, 1},
			{0, 0, 1, 1, 0, 1},
			{0, 1, 0, 1, 0, 1},
			{1, 0, 0, 1, 0, 1},
			{0, 1, 1, 0, 0, 1},
			{1, 0, 1, 0, 0, 1},
			{1, 1, 0, 0, 0, 1},
			{0, 0, 1, 1, 1, 0},
			{0, 1, 0, 1, 1, 0},
			{1, 0, 0, 1, 1, 0},
			{0, 1, 1, 0, 1, 0},
			{1, 0, 1, 0, 1, 0},
			{1, 1, 0, 0, 1, 0},
			{0, 1, 1, 1, 0, 0},
			{1, 0, 1, 1, 0, 0},
			{1, 1, 0, 1, 0, 0},
			{1, 1, 1, 0, 0, 0},
		}
	case 4:
		return [][]int{
			{1, 1, 1, 1, 0, 0},
			{1, 1, 1, 0, 1, 0},
			{1, 1, 0, 1, 1, 0},
			{1, 0, 1, 1, 1, 0},
			{0, 1, 1, 1, 1, 0},
			{1, 1, 1, 0, 0, 1},
			{1, 1, 0, 1, 0, 1},
			{1, 0, 1, 1, 0, 1},
			{0, 1, 1, 1, 0, 1},
			{1, 1, 0, 0, 1, 1},
			{1, 0, 1, 0, 1, 1},
			{0, 1, 1, 0, 1, 1},
			{1, 0, 0, 1, 1, 1},
			{0, 1, 0, 1, 1, 1},
			{0, 0, 1, 1, 1, 1},
		}
	case 5:
		return [][]int{
			{1, 1, 1, 1, 1, 0},
			{1, 1, 1, 1, 0, 1},
			{1, 1, 1, 0, 1, 1},
			{1, 1, 0, 1, 1, 1},
			{1, 0, 1, 1, 1, 1},
			{0, 1, 1, 1, 1, 1},
		}
	case 6:
		return [][]int{
			{1, 1, 1, 1, 1, 1},
		}
	default:
		return nil
	}
}

// returns whether a includes b
func include(as, bs []string) bool {
	if len(bs) > len(as) {
		return false
	}

	i := 0
bloop:
	for i < len(bs) {
		j := 0
		b := bs[i]
		for j < len(as) {
			a := as[j]
			if a == b {
				i++
				continue bloop
			}
			j += 6
		}

		return false
	}

	return true
}
