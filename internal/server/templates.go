package server

import (
	"encoding/json"
	"regexp"

	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/puzzle"
	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/store"
)

var reTime = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

func decodeCrossword(data []byte) (*puzzle.Crossword, error) {
	var c puzzle.Crossword
	err := json.Unmarshal(data, &c)
	return &c, err
}

// Ready-made adventures to start from: the story, the stages with their
// locks and hints, and a setting-up list. Every one can be changed.
type adventureTpl struct {
	kind, title, intro, finale, theme string
	minutes                           int
	prep                              []string
	stages                            []store.Stage
}

func hints(h ...string) []store.Hint {
	out := []store.Hint{}
	for i, t := range h {
		out = append(out, store.Hint{Text: t, After: 3 + i*4})
	}
	return out
}

var adventureTemplates = map[string]adventureTpl{
	"study": {
		kind: "escape", title: "The Professor's Study", theme: "Mystery", minutes: 60,
		intro:  "Professor Vale has vanished, leaving a locked study and a note: “Only a sharp mind may follow me.” You have one hour to find where she went.",
		finale: "The last drawer opens: a train ticket to Vienna and a letter. “Well done. You think like I do. See you at the station.” You escaped with time to spare!",
		prep: []string{"Write the four-digit code (1874) on the back of the old photo", "Lock the desk drawer with the 4-digit padlock",
			"Hide the QR card for stage 3 inside the globe", "Put the letter and the ticket in the last box", "Print the stage cards and the QR cards"},
		stages: []store.Stage{
			{Title: "The note on the desk", Story: "A note on the desk reads: “I am always coming but never arrive. What am I?” The answer opens the cabinet.", Lock: "word", Answers: []string{"tomorrow"},
				LockHint: "A word lock", Hints: hints("Think about time.", "It is never today.", "The answer is TOMORROW."), Hidden: "The note is on the desk, the cabinet has a word lock",
				Unlock: "The cabinet opens. Inside: an old photo with something written on the back."},
			{Title: "The old photo", Story: "On the back of the photo: the year the professor's grandfather built this house. The desk drawer has a four-digit lock.", Lock: "code", Answers: []string{"1874"},
				LockHint: "A 4-digit lock", Hints: hints("Look at the back of the photo again — in a mirror.", "The year is in the 1800s.", "The code is 1874."), Hidden: "Photo in the cabinet; code written backwards on it",
				Unlock: "The drawer slides open. A compass and a card: “The world turns, and so must you.”"},
			{Title: "The world turns", Story: "Something in this room shows the whole world. Find the card hidden in it and scan it.", Lock: "qr", LockHint: "Find the card and scan its code",
				Hints: hints("Something round.", "Spin the globe.", "The card is inside the globe."), Hidden: "QR card inside the globe", Unlock: "On the card: “Up, up, down, left, right.”"},
			{Title: "The direction lock", Story: "The last box has a direction lock. The card told you the way.", Lock: "directions", Answers: []string{"UUDLR"}, LockHint: "Push the lock up, down, left or right",
				Hints: hints("Read the card again.", "Five moves.", "Up, up, down, left, right."), Hidden: "Last box on the shelf", Unlock: ""},
		},
	},
	"birthday": {
		kind: "mystery", title: "The Birthday Mystery", theme: "Birthday", minutes: 45,
		intro:  "Someone has hidden the birthday cake! Follow the clues around the house to find it before the candles are lit.",
		finale: "You found the cake! Happy birthday — now, who wants the first slice?",
		prep:   []string{"Hide the cake (somewhere cool!)", "Put clue 2 in the fridge", "Put the QR card for stage 3 under a sofa cushion", "Wrap a small prize for each guest"},
		stages: []store.Stage{
			{Title: "The first clue", Story: "I have keys but open no doors. Play me and I sing. Where is the next clue?", Lock: "answer", Answers: []string{"piano", "keyboard"},
				Hints: hints("It makes music.", "It has black and white keys."), Hidden: "Clue 2 on the piano / keyboard", Unlock: "The next clue is on the piano!"},
			{Title: "Cold as ice", Story: "Where is it always winter in this house? Look there and type what is written on the clue.", Lock: "word", Answers: []string{"cushion"},
				Hints: hints("It is in the kitchen.", "It keeps the milk cold."), Hidden: "Clue in the fridge says CUSHION", Unlock: "Now look under the cushions!"},
			{Title: "Under the cushions", Story: "Search under the sofa cushions for a card with a code and scan it.", Lock: "qr", Hints: hints("The living room.", "The big sofa."), Hidden: "QR card under a sofa cushion",
				Unlock: "The cake is in the place where coats hang!"},
		},
	},
	"hunt": {
		kind: "hunt", title: "Neighbourhood Treasure Hunt", theme: "Outdoors", minutes: 90,
		intro:  "Walk the neighbourhood, find each place and answer its question. At every stop look for something only someone standing there can see.",
		finale: "Treasure found! Head back home for the prize.",
		prep:   []string{"Walk the route once and check every question", "Pick a prize", "Charge the phones", "Share the link with the teams"},
		stages: []store.Stage{
			{Title: "The fountain", Story: "Go to the fountain in the square. How many fish are on it?", Lock: "code", Answers: []string{"3"}, LockHint: "A number",
				Hints: hints("Look closely at the stone.", "Count again — some are small."), PlaceName: "The square"},
			{Title: "The old bakery", Story: "Find the oldest bakery on the street. What year is painted over the door?", Lock: "code", Answers: []string{"1921"}, LockHint: "A year",
				Hints: hints("Look up, above the door.", "It starts with 19."), PlaceName: "Bakery"},
			{Title: "The park bench", Story: "In the park, find the bench with a name plate. Whose name is on it?", Lock: "answer", Answers: []string{"Ana"},
				Hints: hints("The bench by the pond.", "A short name."), PlaceName: "Park"},
		},
	},
	"kids": {
		kind: "party", title: "Pirate Party Hunt", theme: "Pirates", minutes: 30,
		intro:  "Ahoy, young pirates! Captain Redbeard buried her treasure somewhere nearby. Solve her riddles to find it!",
		finale: "Shiver me timbers — you found the treasure! Share the gold coins fairly, pirates.",
		prep:   []string{"Hide the treasure chest", "Put the QR card under the doormat", "Get chocolate coins"},
		stages: []store.Stage{
			{Title: "The parrot's riddle", Story: "What has a neck but no head? Find one in the kitchen!", Lock: "answer", Answers: []string{"bottle"}, Hints: hints("You drink from it.", "It holds water or juice."),
				Unlock: "Inside the bottle is a map with colours on it: red, blue, red, green."},
			{Title: "The colour chest", Story: "The map shows four colours. Press them in order to open the chest.", Lock: "colors", Answers: []string{"red, blue, red, green"},
				Hints: hints("Look at the map again.", "Red first!"), Unlock: "The chest says: “Look where muddy boots are wiped!”"},
			{Title: "X marks the spot", Story: "Find the card where boots are wiped and scan it.", Lock: "qr", Hints: hints("By the front door."), Hidden: "Under the doormat"},
		},
	},
}

func adventureTemplateList() []map[string]any {
	out := []map[string]any{}
	for _, k := range []string{"study", "birthday", "hunt", "kids"} {
		t := adventureTemplates[k]
		out = append(out, map[string]any{"key": k, "kind": t.kind, "title": t.title, "intro": t.intro, "stages": len(t.stages), "minutes": t.minutes})
	}
	return out
}
