package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"

	pb "github.com/buoyantio/conduit-examples/emojivoto/emojivoto-web/gen/proto"
)

type WebApp struct {
	emojiServiceClient  pb.EmojiServiceClient
	votingServiceClient pb.VotingServiceClient
}

func (app *WebApp) listEmojiHandler(w http.ResponseWriter, r *http.Request) {
	serviceResponse, err := app.emojiServiceClient.ListAll(r.Context(), &pb.ListAllEmojiRequest{})
	if err != nil {
		writeError(err, w, r, http.StatusInternalServerError)
		return
	}

	list := make([]map[string]string, 0)
	for _, e := range serviceResponse.List {
		list = append(list, map[string]string{
			"shortcode": e.Shortcode,
			"unicode":   e.Unicode,
		})
	}

	err = writeJsonBody(w, http.StatusOK, list)

	if err != nil {
		writeError(err, w, r, http.StatusInternalServerError)
	}
}

func (app *WebApp) leaderboardHandler(w http.ResponseWriter, r *http.Request) {
	results, err := app.votingServiceClient.Results(r.Context(), &pb.ResultsRequest{})

	if err != nil {
		writeError(err, w, r, http.StatusInternalServerError)
		return
	}

	representations := make([]map[string]string, 0)
	for _, result := range results.Results {
		findByShortcodeRequest := &pb.FindByShortcodeRequest{
			Shortcode: result.Shortcode,
		}

		findByShortcodeResponse, err := app.emojiServiceClient.FindByShortcode(r.Context(), findByShortcodeRequest)

		if err != nil {
			writeError(err, w, r, http.StatusInternalServerError)
			return
		}

		emoji := findByShortcodeResponse.Emoji
		representation := make(map[string]string)
		representation["votes"] = strconv.Itoa(int(result.Votes))
		representation["unicode"] = emoji.Unicode
		representation["shortcode"] = emoji.Shortcode

		representations = append(representations, representation)
	}

	err = writeJsonBody(w, http.StatusOK, representations)

	if err != nil {
		writeError(err, w, r, http.StatusInternalServerError)
	}
}

func (app *WebApp) voteEmojiHandler(w http.ResponseWriter, r *http.Request) {
	emojiShortcode := r.FormValue("choice")
	if emojiShortcode == "" {
		error := errors.New(fmt.Sprintf("Emoji choice [%s] is mandatory", emojiShortcode))
		writeError(error, w, r, http.StatusBadRequest)
		return
	}

	request := &pb.FindByShortcodeRequest{
		Shortcode: emojiShortcode,
	}
	response, err := app.emojiServiceClient.FindByShortcode(r.Context(), request)
	if err != nil {
		writeError(err, w, r, http.StatusInternalServerError)
		return
	}

	chosenEmoji := response.Emoji
	if chosenEmoji == nil {
		err = errors.New(fmt.Sprintf("Choosen emoji shortcode [%s] doesnt exist", emojiShortcode))
		writeError(err, w, r, http.StatusBadRequest)
		return
	}

	voteRequest := &pb.VoteRequest{
		Shortcode: chosenEmoji.Shortcode,
	}
	_, err = app.votingServiceClient.Vote(r.Context(), voteRequest)
	if err != nil {
		writeError(err, w, r, http.StatusInternalServerError)
		return
	}
}

func (app *WebApp) indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	indexTemplate := `
	<!DOCTYPE html>
	<html>
		<head>
			<meta charset="UTF-8">
		</head>
		<body>
			<div id="main" class="main"></div>
		</body>
		<script type="text/javascript" src="/js" async></script>
	</html>`
	fmt.Fprint(w, indexTemplate)
}

func (app *WebApp) jsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/javascript")
	f, err := ioutil.ReadFile("./index_bundle.js")
	if err != nil {
		panic(err)
	}
	fmt.Fprint(w, string(f))
}

func writeJsonBody(w http.ResponseWriter, status int, body interface{}) error {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(body)
}

func writeError(err error, w http.ResponseWriter, r *http.Request, status int) {
	log.Printf("Error serving request [%v]: %v", r, err)
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(status)
	errorMessage := make(map[string]string)
	errorMessage["error"] = fmt.Sprintf("%v", err)
	json.NewEncoder(w).Encode(errorMessage)
}

func StartServer(webPort string, emojiServiceClient pb.EmojiServiceClient, votingClient pb.VotingServiceClient) {
	webApp := &WebApp{
		emojiServiceClient:  emojiServiceClient,
		votingServiceClient: votingClient,
	}

	log.Printf("Starting web server on WEB_PORT=[%s]", webPort)
	http.HandleFunc("/", webApp.indexHandler)
	http.HandleFunc("/leaderboard", webApp.indexHandler)
	http.HandleFunc("/js", webApp.jsHandler)
	http.HandleFunc("/api/list", webApp.listEmojiHandler)
	http.HandleFunc("/api/vote", webApp.voteEmojiHandler)
	http.HandleFunc("/api/leaderboard", webApp.leaderboardHandler)

	// TODO: make static assets dir configurable
	http.Handle("/dist/", http.StripPrefix("/dist/", http.FileServer(http.Dir("dist"))))

	err := http.ListenAndServe(fmt.Sprintf(":%s", webPort), nil)
	if err != nil {
		panic(err)
	}
}
