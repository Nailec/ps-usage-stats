package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

var ExpectedColumns int = 59
var PlayerIndex int = 0
var TypeIndex int = 1
var LeadIndex int = 2
var PokemonsStart int = 4
var PokemonsColumns int = 9

type CoreStats struct {
	nbUsed         int
	nbWins         int
	nbDeathsPerWin int
	nbDeaths       int
}

type CoresStats map[string]*CoreStats

func (cs CoresStats) String() string {
	res := "{"
	for k, v := range cs {
		res += fmt.Sprintf("%s: %d/%d, ", k, v.nbWins, v.nbUsed)
	}
	return res + "}"
}

func (c1 CoresStats) Merge(c2 CoresStats) {
	for name, cs2 := range c2 {
		cs1, ok := c1[name]
		if !ok {
			c1[name] = cs2
			continue
		}

		cs1.nbUsed += cs2.nbUsed
		cs1.nbWins += cs2.nbWins
		cs1.nbDeathsPerWin += cs2.nbDeathsPerWin
		cs1.nbDeaths += cs2.nbDeaths
	}
}

type StatsFilter struct {
	ForWith        TeamFilter `json:"for_with"`
	ForWithout     TeamFilter `json:"for_without"`
	AgainstWith    TeamFilter `json:"against_with"`
	AgainstWithout TeamFilter `json:"against_without"`
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

type Game struct {
	PokemonPlayerOne []*PokeInfo `json:"pokemon_player_one"`
	PokemonPlayerTwo []*PokeInfo `json:"pokemon_player_two"`
	Winner           string      `json:"winner"`
	PlayerOneName    string      `json:"player_one"`
	PlayerTwoName    string      `json:"player_two"`
}

type PokeInfo struct {
	Name    string         `json:"name"`
	PV      *int           `json:"pv"`
	Attacks map[string]int `json:"attacks"`
}

func (g *Game) String() string {
	res := "{" + g.PlayerOneName + ": ["
	for _, p := range g.PokemonPlayerOne {
		res += p.Name + ", "
	}
	res += "], " + g.PlayerTwoName + ": ["
	for _, p := range g.PokemonPlayerTwo {
		res += p.Name + ", "
	}
	return res + "]}"
}

func main() {
	args := os.Args
	if len(args) != 5 {
		fmt.Println("go run main.go format count output teamFilter")
		return
	}

	format := args[1]
	count, err := strconv.Atoi(args[2])
	if err != nil {
		fmt.Println("cannot parse argument: ", err)
		return
	}

	var output Output
	err = json.Unmarshal([]byte(args[3]), &output)
	if err != nil {
		fmt.Println("cannot parse argument: ", err)
		return
	}

	var filter StatsFilter
	err = json.Unmarshal([]byte(args[4]), &filter)
	if err != nil {
		fmt.Println("cannot parse argument: ", err)
		return
	}

	cs := CoresStats{}
	for retrieved := 0; retrieved < count; retrieved += 100 {
		var games []*Game
		games, err = GetGameInfo(format, retrieved)
		if err != nil {
			fmt.Println(err)
			continue
		}

		games = FilterGames(filter, games)

		cs.Merge(ComputeComboUsage(output, games))
	}
	PrintComboUsage(output, cs)
}

func FilterGames(filter StatsFilter, games []*Game) []*Game {
	res := make([]*Game, len(games))
	i := 0
	for _, game := range games {
		if game.MatchFilter(filter) {
			res[i] = game
			i++
		}
	}

	return res[:i]
}

func GetGameInfo(format string, offset int) ([]*Game, error) {
	resp, err := http.Get("https://www.pkst.net/battle/search?ladder=" + format + "&limit=100&offset=" + strconv.Itoa(offset))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var games []*Game
	err = json.NewDecoder(resp.Body).Decode(&games)
	if err != nil {
		return nil, err
	}

	for _, game := range games {
		game.RemoveDuplicates()
	}

	return games, nil
}

func ComputeComboUsage(output Output, games []*Game) CoresStats {
	combos := allCombo(output.Size)

	cs := CoresStats{}
	for _, game := range games {
		cs.UpdateWithGame(game, combos, output.Size)
	}

	return cs
}

func (cs CoresStats) UpdateWithGame(game *Game, combos [][]int, size int) {
	for _, combo := range combos {
		cs.UpdateWithPlayer(
			game.PokemonPlayerOne,
			game.Winner == game.PlayerOneName,
			combo,
			size,
		)
		cs.UpdateWithPlayer(
			game.PokemonPlayerTwo,
			game.Winner == game.PlayerTwoName,
			combo,
			size,
		)
	}
}

func (cs CoresStats) UpdateWithPlayer(pokemons []*PokeInfo, won bool, combo []int, size int) {
	comboDeathsPerWin := 0
	comboDeaths := 0
	keys := make([]string, size)
	i := 0
	for j, index := range combo {
		if len(pokemons) <= j {
			continue
		}

		if index == 1 {
			keys[i] = pokemons[j].Name
			pokeDeaths := 0
			if pokemons[j].PV != nil && *pokemons[j].PV == 0 {
				pokeDeaths = 1
			}
			if won {
				comboDeathsPerWin += pokeDeaths
			}
			comboDeaths += pokeDeaths
			i++
		}
	}
	if i != size {
		return
	}
	sort.Strings(keys)
	if _, ok := cs[strings.Join(keys, ";")]; !ok {
		cs[strings.Join(keys, ";")] = &CoreStats{}
	}
	cs[strings.Join(keys, ";")].nbUsed++
	cs[strings.Join(keys, ";")].nbDeathsPerWin += comboDeathsPerWin
	cs[strings.Join(keys, ";")].nbDeaths += comboDeaths
	if won {
		cs[strings.Join(keys, ";")].nbWins++
	}
}

func PrintComboUsage(output Output, cs map[string]*CoreStats) {
	// TODO handle leads
	//if output.Lead {
	//	getSpecific(lines, LeadIndex)
	//	return
	//}

	threshold := 10
	if output.Size == 2 || output.Size == 5 {
		threshold = 50
	}

	if output.Size == 3 || output.Size == 4 {
		threshold = 50
	}

	for name, c := range cs {
		if c.nbUsed >= threshold {
			fmt.Println(name + ";" +
				strconv.Itoa(c.nbUsed) + ";" +
				strconv.Itoa(c.nbWins) + ";" +
				strconv.Itoa(c.nbDeathsPerWin) + ";" +
				strconv.Itoa(c.nbDeaths))
		}
	}
}

func (g *Game) MatchFilter(f StatsFilter) bool {
	return (f.ForWith.matchTeam(g.PlayerOneName, g.PokemonPlayerOne) &&
		f.AgainstWith.matchTeam(g.PlayerTwoName, g.PokemonPlayerTwo) ||
		f.AgainstWith.matchTeam(g.PlayerOneName, g.PokemonPlayerOne) &&
			f.ForWith.matchTeam(g.PlayerTwoName, g.PokemonPlayerTwo)) &&
		(!f.ForWithout.matchTeamWithout(g.PlayerOneName, g.PokemonPlayerOne) &&
			!f.AgainstWithout.matchTeamWithout(g.PlayerTwoName, g.PokemonPlayerTwo) ||
			!f.AgainstWithout.matchTeamWithout(g.PlayerOneName, g.PokemonPlayerOne) &&
				!f.ForWithout.matchTeamWithout(g.PlayerTwoName, g.PokemonPlayerTwo))
}

func (g *Game) RemoveDuplicates() {
	g.PokemonPlayerOne = removeDuplicates(g.PokemonPlayerOne)
	g.PokemonPlayerTwo = removeDuplicates(g.PokemonPlayerTwo)
}

func removeDuplicates(pis []*PokeInfo) []*PokeInfo {
	res := make([]*PokeInfo, len(pis))
	i := 0
	sort.SliceStable(pis, func(i, j int) bool {
		return pis[i].Name < pis[j].Name
	})

	for j, pi := range pis {
		pi.Name = cutName(pi.Name)
		if j != 0 && strings.Split(pis[j-1].Name, "-")[0] == strings.Split(pi.Name, "-")[0] {
			res[i-1] = pi
			continue
		}

		res[i] = pi
		i++
	}

	return res[:i]
}

func (f *TeamFilter) matchTeamWithout(player string, pi []*PokeInfo) bool {
	if len(f.Player) != 0 && stringInSliceInsensitive(player, f.Player) {
		return true
	}

	// TODO
	//	if len(f.Lead) != 0 && !stringInSlice(team[LeadIndex], f.Lead) {
	//		return false
	//	}

	if len(f.Type) != 0 {
		pType, err := GetType(pi)
		if err != nil {
			fmt.Println(err)
			return false
		}

		if stringInSlice(pType, f.Type) {
			return true
		}
	}

	if len(f.Pokemons) != 0 && f.pokemonsMatch(pi) {
		return true
	}

	return false
}
func (f *TeamFilter) matchTeam(player string, pi []*PokeInfo) bool {
	if len(f.Player) != 0 && !stringInSliceInsensitive(player, f.Player) {
		return false
	}

	// TODO
	//	if len(f.Lead) != 0 && !stringInSlice(team[LeadIndex], f.Lead) {
	//		return false
	//	}

	if len(f.Type) != 0 {
		pType, err := GetType(pi)
		if err != nil {
			fmt.Println(err)
			return false
		}

		if !stringInSlice(pType, f.Type) {
			return false
		}
	}

	if len(f.Pokemons) != 0 && !f.pokemonsMatch(pi) {
		return false
	}

	return true
}

func (f *TeamFilter) pokemonsMatch(team []*PokeInfo) bool {
	names := make([]string, 0, len(team))
	for _, pi := range team {
		names = append(names, pi.Name)
	}
	for _, andPoke := range f.Pokemons {
		if include(names, andPoke) {
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
			j++
		}

		return false
	}

	return true
}

func cutName(name string) string {
	if strings.HasPrefix(name, "Urshifu") {
		return "Urshifu" // TODO fixme
	}

	if strings.HasPrefix(name, "Mimikyu") {
		return "Mimikyu"
	}

	if strings.HasPrefix(name, "Minior") {
		return "Minior"
	}

	//	if strings.HasPrefix(name, "Pumpkaboo") {
	//		return "Pumpkaboo"
	//	}
	//
	//	if strings.HasPrefix(name, "Gourgeist") {
	//		return "Gourgeist"
	//	}

	if strings.HasPrefix(name, "Toxtricity") {
		return "Toxtricity"
	}

	if strings.HasPrefix(name, "Genesect") {
		return "Genesect"
	}

	if strings.HasPrefix(name, "Eiscue") {
		return "Eiscue"
	}

	if strings.HasPrefix(name, "Sawsbuck") {
		return "Sawsbuck"
	}

	if strings.HasPrefix(name, "Deerling") {
		return "Deerling"
	}

	if strings.HasPrefix(name, "Alcremie") {
		return "Alcremie"
	}

	if strings.HasPrefix(name, "Pikachu") {
		return "Pikachu"
	}

	if strings.HasPrefix(name, "Vivillon") {
		return "Vivillon"
	}

	if strings.HasPrefix(name, "Florges") {
		return "Florges"
	}

	if strings.HasPrefix(name, "Flabebe") {
		return "Flabebe"
	}

	if strings.HasPrefix(name, "Floette") {
		return "Floette"
	}

	if strings.HasPrefix(name, "Furfrou") {
		return "Furfrou"
	}

	if name == "Gastrodon-East" {
		return "Gastrodon"
	}

	if name == "Shellos-East" {
		return "Shellos"
	}

	if name == "Basculin-Blue-Striped" {
		return "Basculin"
	}

	if name == "Polteageist-Antique" {
		return "Polteageist"
	}

	if name == "Keldeo-Resolute" {
		return "Keldeo"
	}

	if strings.HasSuffix(name, "-Totem") {
		return strings.TrimSuffix(name, "-Totem")
	}

	return name
}

type pokeList struct {
	Data []string
}

func GetType(team []*PokeInfo) (string, error) {
	types := []string{"bug", "dark", "dragon", "electric",
		"fairy", "fighting", "fire", "flying", "ghost",
		"grass", "ground", "ice", "normal", "poison",
		"psychic", "rock", "steel", "water",
	}

	var names []string
	for _, t := range types {
		names = make([]string, len(team))
		i := 0

		b, err := ioutil.ReadFile("pokelist/" + t + ".json")
		if err != nil {
			return "", errors.Wrap(err, "could not read type file: "+t)
		}

		var list pokeList
		err = json.Unmarshal(b, &list)
		if err != nil {
			return "", errors.Wrap(err, "could not unmarshal poke list of type: "+t)
		}

		typeMatch := true
		for j, p := range team {
			if p.Name == "" || j != 0 && team[j-1].Name == p.Name {
				continue
			}
			p.Name = cutName(p.Name)
			names[i] = p.Name
			i++
			if strings.HasPrefix(p.Name, "Silvally-") {
				continue
			}
			if strings.HasSuffix(p.Name, "-*") {
				continue
			}
			found := false
			for _, p2 := range list.Data {
				if p.Name == p2 {
					found = true
					break
				}
			}

			if !found {
				typeMatch = false
			}

			if i >= len(team) {
				break
			}
		}

		if typeMatch {
			return t, nil
		}
	}

	return "Unknown", fmt.Errorf("no type found for team: %+v", names)
}
