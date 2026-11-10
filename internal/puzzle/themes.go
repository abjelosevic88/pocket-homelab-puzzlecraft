package puzzle

import "strings"

// Theme is a ready-made word list with clues.
type Theme struct {
	Key      string     `json:"key"`
	Name     string     `json:"name"`
	Language string     `json:"language"`
	Words    []WordClue `json:"words"`
}

func theme(key, name, lang, list string) Theme {
	t := Theme{Key: key, Name: name, Language: lang}
	for _, line := range strings.Split(strings.TrimSpace(list), "\n") {
		w, c, _ := strings.Cut(strings.TrimSpace(line), "|")
		if w = strings.TrimSpace(w); w != "" {
			t.Words = append(t.Words, WordClue{Word: w, Clue: strings.TrimSpace(c)})
		}
	}
	return t
}

// Themes are the built-in word lists.
var Themes = []Theme{
	theme("animals", "Animals", "en", `
ELEPHANT|Largest land animal, with a trunk
GIRAFFE|Tallest animal, long neck
PENGUIN|Bird that cannot fly but swims well
DOLPHIN|Clever sea mammal that clicks and whistles
KANGAROO|Australian animal with a pouch
TIGER|Big striped cat
RABBIT|Long-eared animal that hops
OWL|Night bird that hoots
SQUIRREL|Bushy-tailed nut collector
TURTLE|Slow animal with a shell
ZEBRA|Striped horse of Africa
OCTOPUS|Sea creature with eight arms
HEDGEHOG|Small animal covered in spines
BEAVER|Builds dams in rivers
CAMEL|Desert animal with a hump
FOX|Clever red-furred animal of fairy tales
WOLF|Wild relative of the dog that howls
PARROT|Colourful bird that can copy words`),
	theme("fruit", "Fruit and vegetables", "en", `
APPLE|Fruit that keeps the doctor away
BANANA|Long yellow fruit
CHERRY|Small red fruit with a stone
LEMON|Sour yellow fruit
ORANGE|Fruit and colour
GRAPE|Grows in bunches, makes wine
MELON|Big round summer fruit
PEACH|Fuzzy fruit with a stone
PEAR|Fruit shaped like a bell
PLUM|Purple fruit, dried as a prune
CARROT|Orange root vegetable
POTATO|Vegetable for chips and mash
TOMATO|Red fruit used like a vegetable
ONION|Makes you cry when you cut it
PEPPER|Red, green or yellow, sweet or hot
CUCUMBER|Long green salad vegetable
GARLIC|Strong-smelling bulb in cooking
CABBAGE|Leafy head used for sauerkraut`),
	theme("kitchen", "In the kitchen", "en", `
KETTLE|Boils water for tea
OVEN|Where you bake bread
SPOON|For soup
FORK|Has prongs
KNIFE|For cutting
PLATE|You eat from it
BOWL|For cereal
WHISK|Beats eggs
LADLE|Big spoon for serving soup
GRATER|Turns cheese into shreds
TOASTER|Browns bread slices
FRIDGE|Keeps food cold
SINK|Where you wash up
APRON|Keeps your clothes clean while cooking
PAN|Frying needs one
RECIPE|Instructions for a dish`),
	theme("space", "Space", "en", `
PLANET|Earth is one
STAR|The Sun is one
MOON|It goes round the Earth
COMET|Icy visitor with a tail
ROCKET|It takes astronauts up
GALAXY|The Milky Way is one
ORBIT|Path round a planet
MARS|The red planet
JUPITER|The largest planet
SATURN|Planet with famous rings
VENUS|The hottest planet
METEOR|Shooting star
ASTRONAUT|Space traveller
TELESCOPE|For looking at the stars
ECLIPSE|When the Moon hides the Sun
NEBULA|Cloud of gas where stars are born`),
	theme("weather", "Weather", "en", `
RAIN|Falls from clouds
SNOW|White and cold
THUNDER|Boom after lightning
LIGHTNING|Flash in a storm
RAINBOW|Seven colours after rain
CLOUD|Grey or white in the sky
WIND|Moving air
FOG|Thick low cloud
HAIL|Balls of ice from the sky
FROST|Ice on the grass in the morning
SUNSHINE|Bright warm weather
STORM|Wild weather
BREEZE|Gentle wind
DRIZZLE|Very light rain
TORNADO|Spinning funnel of wind`),
	theme("sports", "Sports", "en", `
FOOTBALL|Game with a goal at each end
TENNIS|Played with rackets over a net
BASKETBALL|Ball through a hoop
SWIMMING|Sport in a pool
CYCLING|Sport on two wheels
SKIING|Down a snowy slope
GOLF|Hole in one
RUGBY|Oval ball game
CHESS|Game of kings and pawns
BOXING|Sport in a ring with gloves
VOLLEYBALL|Ball over a high net, by hand
ROWING|Racing boats with oars
HOCKEY|Played with sticks
MARATHON|Race of about 42 kilometres
REFEREE|Keeps the rules in a match`),
	theme("music", "Music", "en", `
GUITAR|Six strings
PIANO|Black and white keys
VIOLIN|Played with a bow under the chin
DRUM|You hit it
FLUTE|Woodwind you blow across
TRUMPET|Brass with three valves
MELODY|The tune
RHYTHM|The beat
CHORD|Notes played together
CHOIR|Group of singers
CONCERT|Live show
TEMPO|How fast music goes
ORCHESTRA|Big group of players
LYRICS|The words of a song
HARMONY|Notes that sound well together`),
	theme("countries", "Countries", "en", `
FRANCE|Home of the Eiffel Tower
ITALY|Shaped like a boot
SPAIN|Flamenco and paella
JAPAN|Land of the rising sun
BRAZIL|Biggest country in South America
EGYPT|Pyramids and the Nile
CANADA|Maple leaf on its flag
INDIA|Taj Mahal
CHINA|Great Wall
NORWAY|Fjords
GREECE|Birthplace of the Olympics
MEXICO|Tacos and mariachi
KENYA|Safari in East Africa
PERU|Machu Picchu
BOSNIA|Mostar's old bridge
CROATIA|Dubrovnik on the Adriatic`),
	theme("home", "Around the house", "en", `
WINDOW|You look out of it
DOOR|You open it to go in
STAIRS|They take you up
ROOF|Top of the house
SOFA|Comfy seat for three
LAMP|Gives light
MIRROR|Shows your face
PILLOW|For your head in bed
CARPET|Covers the floor
GARDEN|Where flowers grow
CHIMNEY|Smoke goes up it
CURTAIN|Closes over a window
BATHTUB|You soak in it
DOORBELL|Rings when someone visits
BOOKSHELF|Holds your books`),
	theme("ocean", "Under the sea", "en", `
WHALE|Biggest animal ever
SHARK|Fish with many rows of teeth
CORAL|Reef builder
SEAHORSE|Fish shaped like a chess knight
CRAB|Walks sideways
JELLYFISH|Stings and floats
STARFISH|Five-armed sea creature
SEAWEED|Plant of the sea
OYSTER|Makes pearls
LOBSTER|Big claws, red when cooked
WAVE|Rolls onto the beach
TIDE|Comes in and goes out twice a day
SHELL|Washed up on the sand
SUBMARINE|Boat that goes under water
ANCHOR|Holds a ship in place`),
	theme("feelings", "Feelings", "en", `
HAPPY|Feeling good
SAD|Feeling down
ANGRY|Feeling cross
BRAVE|Not afraid
CALM|Peaceful
PROUD|Pleased with what you did
NERVOUS|Butterflies in your tummy
EXCITED|Can't wait
CURIOUS|Wants to know
GRATEFUL|Thankful
LONELY|Missing company
SURPRISED|Didn't see that coming
JEALOUS|Wanting what someone else has
CONFUSED|Not sure what is going on`),
	theme("birthday", "Birthday party", "en", `
CAKE|Has candles on top
CANDLES|You blow them out
PRESENT|Wrapped gift
BALLOON|Filled with air or helium
PARTY|A celebration
GAMES|Played by the guests
CARD|Says happy birthday
SONG|Everyone sings it
WISH|Make one before you blow
FRIENDS|Your guests
CONFETTI|Tiny paper pieces thrown
INVITATION|Asks people to come
DANCE|Moving to the music
SURPRISE|A party they did not expect`),
	theme("school", "School", "en", `
TEACHER|Leads the class
PENCIL|Write with it, rub it out
RULER|Measures and draws straight lines
BACKPACK|Carries your books
LESSON|One class period
HOMEWORK|Work to do at home
LIBRARY|Room full of books
SCIENCE|Experiments and discoveries
HISTORY|The story of the past
MATHS|Numbers and sums
GEOGRAPHY|Maps and places
EXAM|Big test
BREAK|Time to play between lessons
CLASSROOM|Where lessons happen`),
	theme("zivotinje", "Životinje (BCS)", "bcs", `
MAČKA|Kućni ljubimac koji prede
PAS|Čovjekov najbolji prijatelj
KONJ|Na njemu se jaše
KRAVA|Daje mlijeko
KORNJAČA|Nosi kuću na leđima
ŽABA|Kvače u bari
MEDVJED|Spava zimski san
VUK|Zavija na mjesec
LISICA|Lukava crvenka
ZEC|Ima duge uši
JEŽ|Bodljikav
ORAO|Kralj ptica
ŠARAN|Riba iz rijeke
PČELA|Pravi med
DŽUKELA|Pas bez rase
SOVA|Noćna ptica
NJORKA|Morska ptica ronilica
SLON|Ima surlu`),
	theme("voce", "Voće i povrće (BCS)", "bcs", `
JABUKA|Crvena ili zelena, za pitu
KRUŠKA|Voće oblika zvona
ŠLJIVA|Od nje je pekmez i rakija
TREŠNJA|Mala crvena, s košticom
LUBENICA|Velika, zelena izvana, crvena iznutra
DINJA|Žuta i slatka ljeti
GROŽĐE|Raste u grozdovima
NARANDŽA|Narandžasto južno voće
LIMUN|Kiselo žuto voće
PAPRIKA|Za ajvar
KRASTAVAC|Za salatu i kisele krastavce
KRUMPIR|Za pomfrit
LUK|Od njega se plače
MRKVA|Narandžasti korijen
KUPUS|Za sarmu`),
	theme("grad", "U gradu (BCS)", "bcs", `
ULICA|Ide se njom
TRG|Veliki otvoreni prostor u centru
PIJACA|Tu se kupuje voće
ŠKOLA|Tu su učenici
BOLNICA|Tu rade doktori
POŠTA|Šalje pisma
BIBLIOTEKA|Puna knjiga
DŽAMIJA|Ima minaret
CRKVA|Ima zvonik
MOST|Preko rijeke
TRAMVAJ|Vozi po šinama
PARK|Zelenilo u gradu
KAFANA|Tu se pije kafa
APOTEKA|Prodaje lijekove`),
}

// ThemeByKey finds a built-in word list.
func ThemeByKey(k string) (Theme, bool) {
	for _, t := range Themes {
		if t.Key == k {
			return t, true
		}
	}
	return Theme{}, false
}

// Quote is a text for a secret code.
type Quote struct {
	Text   string `json:"text"`
	Author string `json:"author"`
}

// Quotes are old sayings and quotes from long ago, free to use.
var Quotes = []Quote{
	{"The journey of a thousand miles begins with one step.", "Lao Tzu"},
	{"Knowing yourself is the beginning of all wisdom.", "Aristotle"},
	{"Well done is better than well said.", "Benjamin Franklin"},
	{"Do not count your chickens before they are hatched.", "Aesop"},
	{"Slow and steady wins the race.", "Aesop"},
	{"The only true wisdom is in knowing you know nothing.", "Socrates"},
	{"Happiness depends upon ourselves.", "Aristotle"},
	{"It does not matter how slowly you go as long as you do not stop.", "Confucius"},
	{"Wherever you go, go with all your heart.", "Confucius"},
	{"Little by little, one travels far.", "J. R. R. Tolkien"},
	{"An investment in knowledge pays the best interest.", "Benjamin Franklin"},
	{"Be yourself; everyone else is already taken.", "Oscar Wilde"},
	{"We are what we repeatedly do.", "Will Durant"},
	{"Hope is a waking dream.", "Aristotle"},
	{"The best way out is always through.", "Robert Frost"},
	{"Not all those who wander are lost.", "J. R. R. Tolkien"},
	{"Imagination is more important than knowledge.", "Albert Einstein"},
	{"Life is really simple, but we insist on making it complicated.", "Confucius"},
	{"No act of kindness, no matter how small, is ever wasted.", "Aesop"},
	{"Where there is love there is life.", "Mahatma Gandhi"},
	{"The secret of getting ahead is getting started.", "Mark Twain"},
	{"Courage is grace under pressure.", "Ernest Hemingway"},
	{"A friend to all is a friend to none.", "Aristotle"},
	{"Nothing will come of nothing.", "William Shakespeare"},
	{"All the world's a stage.", "William Shakespeare"},
	{"Brevity is the soul of wit.", "William Shakespeare"},
	{"To be or not to be, that is the question.", "William Shakespeare"},
	{"Fortune favours the bold.", "Virgil"},
	{"Time flies like an arrow; fruit flies like a banana.", "Old joke"},
	{"Early to bed and early to rise makes a man healthy, wealthy and wise.", "Benjamin Franklin"},
	{"Tell me and I forget. Teach me and I remember. Involve me and I learn.", "Old saying"},
	{"Many hands make light work.", "Old saying"},
	{"Actions speak louder than words.", "Old saying"},
	{"Every cloud has a silver lining.", "Old saying"},
	{"Bez muke nema nauke.", "Narodna poslovica"},
	{"Ko rano rani, dvije sreće grabi.", "Narodna poslovica"},
	{"Strpljen, spašen.", "Narodna poslovica"},
}

// Riddle is a classic riddle.
type Riddle struct {
	Q, A string
}

// Riddles are old folk riddles.
var Riddles = []Riddle{
	{"What has keys but cannot open locks?", "a piano"},
	{"What has to be broken before you can use it?", "an egg"},
	{"What gets wetter the more it dries?", "a towel"},
	{"What has a neck but no head?", "a bottle"},
	{"What goes up but never comes down?", "your age"},
	{"What has many teeth but cannot bite?", "a comb"},
	{"What can you catch but not throw?", "a cold"},
	{"What has hands but cannot clap?", "a clock"},
	{"The more you take, the more you leave behind. What are they?", "footsteps"},
	{"What runs but never walks, has a mouth but never talks?", "a river"},
	{"What has one eye but cannot see?", "a needle"},
	{"What is full of holes but still holds water?", "a sponge"},
	{"I am tall when I am young and short when I am old. What am I?", "a candle"},
	{"What comes once in a minute, twice in a moment, but never in a thousand years?", "the letter m"},
	{"What can travel around the world while staying in a corner?", "a stamp"},
	{"What has a head and a tail but no body?", "a coin"},
	{"What building has the most stories?", "a library"},
	{"What has legs but does not walk?", "a table"},
	{"What belongs to you, but others use it more than you do?", "your name"},
	{"What can fill a room but takes up no space?", "light"},
}

// LogicSet is a bank of categories for making logic grids.
type LogicSet struct {
	Key   string
	Name  string
	Story string
	Cats  []Category
}

// LogicSets are ready-made categories (each category lists more items
// than needed; a grid picks some).
var LogicSets = []LogicSet{
	{"pets", "Pets and owners", "Some friends each have a pet with a different name and a favourite treat. Work out who has which.", []Category{
		{Name: "Owner", Items: []string{"Ana", "Ben", "Clara", "David", "Emma", "Filip"}},
		{Name: "Pet", Items: []string{"cat", "dog", "parrot", "rabbit", "hamster", "turtle"}},
		{Name: "Name", Items: []string{"Biscuit", "Pepper", "Luna", "Max", "Ziggy", "Coco"}},
		{Name: "Treat", Items: []string{"carrot", "cheese", "seeds", "fish", "apple", "biscuit"}},
	}},
	{"race", "The fun run", "Runners finished a fun run one after the other, each in a different coloured shirt. Who came where?", []Category{
		{Name: "Place", Items: []string{"1st", "2nd", "3rd", "4th", "5th", "6th"}, Ordered: true},
		{Name: "Runner", Items: []string{"Mia", "Noah", "Lena", "Omar", "Iva", "Tom"}},
		{Name: "Shirt", Items: []string{"red", "blue", "green", "yellow", "white", "orange"}},
		{Name: "Town", Items: []string{"Mostar", "Tuzla", "Zenica", "Bihać", "Banja Luka", "Sarajevo"}},
	}},
	{"cafe", "Breakfast at the café", "Friends met for breakfast at different times; each ordered a drink and a pastry.", []Category{
		{Name: "Time", Items: []string{"7:00", "7:30", "8:00", "8:30", "9:00", "9:30"}, Ordered: true},
		{Name: "Friend", Items: []string{"Sara", "Marko", "Lejla", "Ivan", "Nina", "Adam"}},
		{Name: "Drink", Items: []string{"coffee", "tea", "juice", "cocoa", "milk", "water"}},
		{Name: "Pastry", Items: []string{"croissant", "burek", "muffin", "bagel", "donut", "scone"}},
	}},
	{"holiday", "Summer holidays", "Families went on holiday to different places, for a different number of nights.", []Category{
		{Name: "Family", Items: []string{"Horvat", "Kovač", "Smith", "Novak", "Petrović", "Müller"}},
		{Name: "Place", Items: []string{"Split", "Rome", "Vienna", "Paris", "Prague", "Athens"}},
		{Name: "Nights", Items: []string{"3", "4", "5", "6", "7", "8"}, Ordered: true},
		{Name: "Travel", Items: []string{"car", "train", "plane", "bus", "boat", "bike"}},
	}},
	{"garden", "The garden club", "Gardeners each grew a prize vegetable of a different weight.", []Category{
		{Name: "Gardener", Items: []string{"Rosa", "Hugo", "Vera", "Leo", "Maja", "Igor"}},
		{Name: "Vegetable", Items: []string{"pumpkin", "marrow", "cabbage", "beetroot", "onion", "leek"}},
		{Name: "Weight", Items: []string{"2 kg", "3 kg", "4 kg", "5 kg", "6 kg", "7 kg"}, Ordered: true},
		{Name: "Prize", Items: []string{"cup", "medal", "ribbon", "voucher", "book", "plant"}},
	}},
}

// LogicSetByKey finds a category bank.
func LogicSetByKey(k string) (LogicSet, bool) {
	for _, s := range LogicSets {
		if s.Key == k {
			return s, true
		}
	}
	return LogicSet{}, false
}
