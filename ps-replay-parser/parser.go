package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"runtime/debug"
	"strings"

	"github.com/pkg/errors"
)

type Team struct {
	Pokemons       map[string]*Pokemon // Key is Nickname
	Lead           string
	Result         string
	Player         string
	Type           string
	DynamaxPokemon string
	DynamaxTurn    int
	BattleLength   int
}

type Pokemon struct {
	Name      string
	Moves     []string
	Item      string
	Kills     int
	Deaths    int // only 0 or 1
	Entrances int
}

func GetURLsFromFile(file, format string) ([]string, error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	fileContent := string(b)

	lines := strings.Split(fileContent, "\n")
	urls := make([]string, len(lines))
	i := 0

	for _, line := range lines {
		if strings.HasPrefix(line, "replay") {
			line = strings.Replace(line, "replay", "https://replay", 1)
		}
		line = strings.Replace(line, "http://", "https://", 1)
		if !strings.HasPrefix(line, "https://replay") {
			continue
		}

		urls[i] = line
		i++
	}

	return urls[:i], nil
}

func GetTeams(paths []string, format string, isLogs bool) ([]*Team, error) {
	allTeams := make([]*Team, 2*len(paths))
	teams := make(map[string]*Team, 2)
	i := 0
	var err error
	var errs []error
	for _, path := range paths {
		if isLogs {
			teams, err = ParsePokemonsFromFile(path)
		} else {
			teams, err = ParsePokemonsFromURL(path)
		}
		if err != nil {
			return nil, err
		}

		for _, team := range teams {
			if strings.Contains(format, "monotype") {
				team.Type, err = GetType(team.Pokemons)
				if err != nil {
					errs = append(errs, err)
				}
			}
			allTeams[i] = team
			i++
		}
	}

	if len(errs) != 0 {
		for _, err := range errs {
			fmt.Println(err)
		}
		return nil, errors.New("errors occured when trying to find team types")
	}

	return allTeams, nil
}

func GetStats(paths []string, isLogs bool) (map[string]int, error) {
	stats := map[string]int{}
	teams := make(map[string]*Team, 2)
	var err error
	var teamType string
	for _, path := range paths {
		if isLogs {
			teams, err = ParsePokemonsFromFile(path)
		} else {
			teams, err = ParsePokemonsFromURL(path)
		}
		if err != nil {
			return nil, err
		}

		for _, team := range teams {
			teamType, err = GetType(team.Pokemons)
			if err != nil {
				return nil, err
			}

			for _, pokemon := range team.Pokemons {
				stats[pokemon.Name+"\t"+teamType]++
			}
		}
	}

	return stats, nil
}

type pokeList struct {
	Data []string `json:"data"`
}

func GetType(team map[string]*Pokemon) (string, error) {
	types := []string{"bug", "dark", "dragon", "electric",
		"fairy", "fighting", "fire", "flying", "ghost",
		"grass", "ground", "ice", "normal", "poison",
		"psychic", "rock", "steel", "water",
	}

	var names []string
	for _, t := range types {
		names = make([]string, 6)
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
		for _, p := range team {
			names[i] = p.Name
			i++
			if strings.HasPrefix(p.Name, "Silvally-") {
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
		}

		if typeMatch {
			return t, nil
		}
	}

	return "Unknown", fmt.Errorf("no type found for team: %+v", names)
}

func ParsePokemonsFromFile(file string) (map[string]*Team, error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(file)
		}
	}()

	b, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	return ParsePokemonsFromHtml(string(b))
}

func ParsePokemonsFromURL(url string) (map[string]*Team, error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(url)
			fmt.Println(string(debug.Stack()))
		}
	}()

	resp, err := http.Get(url + ".log")
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("could not access: %s, code: %d",
			url, resp.StatusCode)
	}

	defer resp.Body.Close()
	html, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if strings.Contains(string(html), "Could not connect") {
		return nil, fmt.Errorf("could not connect to: %s", url)
	}

	return ParsePokemonsFromHtml(string(html))
}

func ParsePokemonsFromHtml(html string) (map[string]*Team, error) {
	teams := map[string]*Team{
		"p1": &Team{
			Result:   "L",
			Pokemons: map[string]*Pokemon{},
		},
		"p2": &Team{
			Result:   "L",
			Pokemons: map[string]*Pokemon{},
		},
	} // The teams to be returned

	playerIDs := map[string]string{}     // Stores a player ID by name
	playerCurrent := map[string]string{} // Stores the pokemon on this player's side
	turn := 0

	lines := strings.Split(html, "\n")
	for i, line := range lines {
		// Init turn
		if strings.HasPrefix(line, "|turn|") {
			turn++
			continue
		}

		// Init players
		if strings.HasPrefix(line, "|player|") {
			split := strings.Split(line, "|")
			playerIDs[split[3]] = split[2]
			teams[split[2]].Player = split[3]
			continue
		}

		// Init pokemons
		if strings.HasPrefix(line, "|poke|") {
			split := strings.Split(line, "|")
			p := split[2]
			poke := strings.Split(split[3], ",")[0]
			poke = cutName(poke)

			if poke == "Greninja" {
				nn := GetNickname(html, p, poke)
				if checkAshGreninja(html, p, nn) {
					teams[p].Pokemons[poke] = &Pokemon{
						Name:  "Greninja-Ash",
						Moves: make([]string, 4),
					}
					continue
				}
			}

			// Pokemon is initialized with its base name as nickname
			teams[p].Pokemons[poke] = &Pokemon{
				Name:  poke,
				Moves: make([]string, 4),
			}
			continue
		}

		// Init leads
		if strings.HasPrefix(line, "|start") {
			if strings.HasSuffix(line, "Dynamax") {
				pID, pokeNick := getDynamaxInfo(line)
				teams[pID].DynamaxPokemon = pokeNick
				teams[pID].DynamaxTurn = turn
				continue
			}

			pID, pokeNick, pokeName := getPoke(lines[i+1])
			pokeName = cutName(pokeName)
			teams[pID].Lead = pokeNick
			updatePlayerPoke(teams[pID].Pokemons, pokeNick, pokeName)
			teams[pID].Pokemons[pokeNick].Entrances++
			if pokeNick != pokeName {
				delete(teams[pID].Pokemons, pokeName)
			}

			pID, pokeNick, pokeName = getPoke(lines[i+2])
			pokeName = cutName(pokeName)
			teams[pID].Lead = pokeNick
			updatePlayerPoke(teams[pID].Pokemons, pokeNick, pokeName)
			teams[pID].Pokemons[pokeNick].Entrances++
			if pokeNick != pokeName {
				delete(teams[pID].Pokemons, pokeName)
			}

			i += 2
			continue
		}

		// Dynamax
		if strings.HasPrefix(line, "|-start") && strings.HasSuffix(line, "Dynamax") {
			pID, pokeNick := getDynamaxInfo(line)
			teams[pID].DynamaxPokemon = pokeNick
			teams[pID].DynamaxTurn = turn
			continue
		}

		// Item detection
		if strings.Contains(line, "[from] item: ") {
			pID, pokeNick, item := getItem(line)
			teams[pID].Pokemons[pokeNick].Item = item
		}

		if strings.HasPrefix(line, "|-enditem") {
			pID, pokeNick, item := getItemFromEndItem(line)
			teams[pID].Pokemons[pokeNick].Item = item
			continue
		}

		// Handle end of battle result
		if strings.HasPrefix(line, "|win") {
			split := strings.Split(line, "|")
			teams[playerIDs[split[2]]].Result = "W"
			for _, team := range teams {
				team.BattleLength = turn
			}
			break // nothing is interesting after we know who won
		}

		// Update nickname and details on forms (silvally, pumpkaboo, ...)
		if strings.HasPrefix(line, "|switch") || strings.HasPrefix(line, "|drag") {
			pID, pokeNick, pokeName := getPoke(line)
			playerCurrent[pID] = pokeNick
			if _, ok := teams[pID].Pokemons[pokeNick]; ok {
				teams[pID].Pokemons[pokeNick].Entrances++
				continue // already checked ^^
			}

			pokeName = cutName(pokeName)
			updatePlayerPoke(teams[pID].Pokemons, pokeNick, pokeName)
			teams[pID].Pokemons[pokeNick].Entrances++
			playerCurrent[pID] = pokeNick
			if pokeNick != pokeName {
				delete(teams[pID].Pokemons, pokeName)
			}
			continue
		}

		if strings.HasPrefix(line, "|faint") {
			pID, pokeNick := getFaintInfo(line)
			opp := getOpp(pID)
			teams[opp].Pokemons[playerCurrent[opp]].Kills++
			teams[pID].Pokemons[pokeNick].Deaths++
		}

		// Update form detail
		if strings.HasPrefix(line, "|detailschange") {
			pID, pokeNick, pokeName := getDetailschange(line)
			teams[pID].Pokemons[pokeNick].Name = pokeName
			continue
		}

		// |move|p1a: Liepard|Taunt||[from]Copycat|[still]
		if strings.HasPrefix(line, "|move") {
			if strings.Contains(line, "[zeffect]") {
				continue
			}
			if strings.Contains(line, "[from]Magic Bounce") {
				continue
			}
			if strings.Contains(line, "[from]Metronome") {
				continue
			}
			if strings.Contains(line, "[from]Assist") {
				continue
			}
			if strings.Contains(line, "[from]Snatch") {
				continue
			}
			if strings.Contains(line, "[from]Magic Coat") {
				continue
			}
			if strings.Contains(line, "[from]Nature Power") {
				continue
			}
			if strings.Contains(line, "[from]Me First") {
				continue
			}
			if strings.Contains(line, "[from]Copycat") {
				continue
			}

			pID, pokeNick, move := getMoveInfo(line)
			if strings.HasPrefix(move, "Z-") {
				move = strings.TrimPrefix(move, "Z-")
			}
			if move == "Struggle" {
				continue
			}
			if strings.HasPrefix(move, "Max ") || strings.HasPrefix(move, "G-Max ") {
				continue
			}
			if teams[pID].Pokemons[pokeNick].Name == "Ditto" {
				continue
			}
			addMove(teams[pID].Pokemons[pokeNick].Moves, move)
			continue
		}

		// |cant|p2a: Clefable|move: Taunt|Stealth Rock
		if strings.HasPrefix(line, "|cant") {
			pID, pokeNick, move := getCantMoveInfo(line)
			if pID == "" {
				continue
			}
			if teams[pID].Pokemons[pokeNick].Name == "Ditto" {
				continue
			}
			addMove(teams[pID].Pokemons[pokeNick].Moves, move)
			continue
		}

		if strings.HasPrefix(line, "|-zpower") {
			pID, pokeNick, move := getMoveInfo(lines[i+1])
			i++
			teams[pID].Pokemons[pokeNick].Item = move
			continue
		}
	}

	return teams, nil
}

func GetNickname(html, player, poke string) string {
	expected := regexp.MustCompile(`\|switch\|` + player + `a: ([^\|]*)\|` + poke)

	res := expected.FindStringSubmatch(html)
	if len(res) < 2 {
		return poke
	}

	return res[1]
}

func checkAshGreninja(html, player, nn string) bool {
	protean := regexp.MustCompile(`\|-start\|` + player + `a: ` + nn + `\|typechange\|([^\|]*)\|\[from\] (ability: )?Protean`)

	usedMove := regexp.MustCompile(`\|move\|` + player + `a: ` + nn + `\|([^\|]*)\|`)

	return strings.Contains(html, "|detailschange|"+player+"a: "+nn+"|"+"Greninja-Ash") || !protean.MatchString(html) && usedMove.MatchString(html)
}

// returns player, nickname and name
func getPoke(line string) (string, string, string) {
	expected := regexp.MustCompile(`\|(switch|drag)\|(p(1|2))a: ([^\|]*)\|([^,|]*)`)

	res := expected.FindStringSubmatch(line)
	return res[2], res[4], res[5]
}

func getOpp(p string) string {
	if p == "p1" {
		return "p2"
	}

	return "p1"
}

// returns player and nickname
func getFaintInfo(line string) (string, string) {
	expected := regexp.MustCompile(`\|faint\|(p(1|2))a: ([^\|]*)$`)

	res := expected.FindStringSubmatch(line)
	return res[1], res[3]
}

// returns player, nickname and item
func getItemFromEndItem(line string) (string, string, string) {
	expected := regexp.MustCompile(`\|-enditem\|(p(1|2))a: ([^\|]*)\|([^\|]*)(.*)$`)

	res := expected.FindStringSubmatch(line)
	return res[1], res[3], res[4]
}

// returns player, nickname and item
func getItem(line string) (string, string, string) {
	expected := regexp.MustCompile(`\|-(damage|heal|status|boost|unboost)\|(p(1|2))a: ([^\|]*)\|([^\[]*)\[from\] item: ([^\|]*)(\|\[of\] (p(1|2))a: ([^\|]*))?$`) // fixme ?

	res := expected.FindStringSubmatch(line)
	if len(res) < 10 {
		fmt.Println(line, res)
	}
	if res[10] != "" {
		return res[8], res[10], res[6]
	}

	return res[2], res[4], res[6]
}

// returns player, nickname and move
func getMoveInfo(line string) (string, string, string) {
	expected := regexp.MustCompile(`\|move\|(p(1|2))a: ([^\|]*)\|([^,|]*)`)

	res := expected.FindStringSubmatch(line)
	return res[1], res[3], res[4]
}

// returns player, nickname
func getDynamaxInfo(line string) (string, string) {
	expected := regexp.MustCompile(`\|-start\|(p(1|2))a: ([^\|]*)\|Dynamax`)

	res := expected.FindStringSubmatch(line)
	return res[1], res[3]
}

// |cant|p2a: Clefable|move: Taunt|Stealth Rock
// returns player, move and name
func getCantMoveInfo(line string) (string, string, string) {
	expected := regexp.MustCompile(`\|cant\|(p(1|2))a: ([^\|]*)\|move: ([^,|]*)\|([^\|]*)$`)

	res := expected.FindStringSubmatch(line)
	if len(res) == 0 {
		return "", "", ""
	}
	return res[1], res[3], res[5]
}

// returns player, nickname and new form name
func getDetailschange(line string) (string, string, string) {
	expected := regexp.MustCompile(`\|detailschange\|(p(1|2))a: ([^\|]*)\|([^,|]*)`)

	res := expected.FindStringSubmatch(line)

	if strings.HasPrefix(res[4], "Mimikyu") {
		res[4] = "Mimikyu"
	}

	if strings.HasPrefix(res[4], "Toxtricity") {
		res[4] = "Toxtricity"
	}

	if strings.HasPrefix(res[4], "Genesect") {
		res[4] = "Genesect"
	}

	if strings.HasPrefix(res[4], "Eiscue") {
		res[4] = "Eiscue"
	}

	if strings.HasPrefix(res[4], "Minior") {
		res[4] = "Minior"
	}

	return res[1], res[3], res[4]
}

func updatePlayerPoke(pokes map[string]*Pokemon, nick, newName string) {
	matched := false
	for oldName, poke := range pokes {
		if !namesMatch(newName, oldName) {
			continue
		}

		matched = true
		// we already have it in current or another form and it's not an annoying form
		if newName != "Silvally" && newName != "Gourgeist" && newName != "Pumpkaboo" &&
			(nick == oldName || nick == newName) {
			break
		}
		poke.Name = newName
		pokes[nick] = poke
		delete(pokes, oldName)
		break
	}

	if !matched {
		fmt.Println("WTF is " + newName + ", DPP ?")
		pokes[nick] = &Pokemon{
			Name:  newName,
			Moves: make([]string, 4),
		}
	}
}

func namesMatch(a, b string) bool {
	if a == b {
		return true
	}

	as := strings.Split(a, "-")
	bs := strings.Split(b, "-")
	return as[0] == bs[0]
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

func addMove(a []string, s string) {
	for i, v := range a {
		if s == v {
			return
		}
		if v == "" {
			a[i] = s
			return
		}
	}
	fmt.Println("cannot add: " + s + " to: " + strings.Join(a, ","))
}
