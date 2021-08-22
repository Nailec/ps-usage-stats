# ps-replay-parser
A tool to parse replays of pokemon battles !

This programs takes the following parameters : 
 * address # the location of the file containing the replay links
 * format # the format of the battles (useful to filter out a gen in a tour for example)
 * output_type # If teams returns a csv of the teams with the format below. If stats returns the usage of each pokemon+type combination (monotype only)

examples on how to run the program : <br>
go run *.go ~/lcuu_replays gen7lcuu teams

teams output format : <br>
`player_name;team_type;lead;battle_length;pokemon1;item;move1;move2;move3;move4;kills;deaths;switch-ins;pokemon2;(...);pokemon6;item;move1;move2;move3;move4;kills;deaths;switch-ins;result` # result is W or L
