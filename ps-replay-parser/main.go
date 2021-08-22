package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func main() {
	args := os.Args
	if len(args) != 4 {
		fmt.Println("go run main.go filename format stats/teams")
		return
	}

	format := args[2]

	fileInfo, err := os.Stat(args[1])
	if err != nil {
		fmt.Println(err)
		return
	}

	isLogs := fileInfo.IsDir()

	var paths []string
	if isLogs {
		files, err := ioutil.ReadDir(args[1])
		if err != nil {
			fmt.Println(err)
			return
		}

		paths = make([]string, len(files))
		for i, file := range files {
			paths[i] = filepath.Join(args[1], file.Name())
		}
	} else {
		paths, err = GetURLsFromFile(args[1], format)
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	switch args[3] {
	case "stats":
		res, err := GetStats(paths, isLogs)
		if err != nil {
			fmt.Println(err)
			return
		}

		for name, val := range res {
			fmt.Printf(name + "\t" + strconv.Itoa(val) + "\n")
		}
	case "teams":
		res, err := GetTeams(paths, format, isLogs)
		if err != nil {
			fmt.Println(err)
			return
		}

		for _, team := range res {
			displayTeam(team)
		}
	}
}

func displayTeam(team *Team) {
	if team == nil || team.Lead == "" {
		return
	}

	pokes := make([]string, len(team.Pokemons))
	i := 0
	for _, poke := range team.Pokemons {
		pokes[i] = poke.Name
		i++
	}

	leadmon := team.Lead
	if _, ok := team.Pokemons[team.Lead]; ok {
		leadmon = team.Pokemons[team.Lead].Name
	}

	sort.Strings(pokes)
	output := team.Player + ";"
	output += team.Type + ";"
	output += leadmon + ";"
	output += strconv.Itoa(team.BattleLength) + ";"
	for _, poke := range pokes {
		for _, p := range team.Pokemons {
			if poke == p.Name {
				output += poke + ";"
				output += p.Item + ";"
				output += strings.Join(p.Moves, ";") + ";"
				output += strconv.Itoa(p.Kills) + ";"
				output += strconv.Itoa(p.Deaths) + ";"
				output += strconv.Itoa(p.Entrances) + ";"
			}
		}
	}
	for i < 6 {
		output += ";;;;;;"
		i++
	}
	output += team.Result
	fmt.Println(output)
}
