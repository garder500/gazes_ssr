package routes

import (
	"bufio"
	"encoding/json"
	"fmt"
	"gazes_ssr/functions"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func DownloadHandler(w http.ResponseWriter, r *http.Request, id, ep string, cache *functions.Cache) {
	// before returning the html, we need to replace the {{meta}} with the actual metadata
	datas, err := getAnime(id)
	if err != nil {
		log.Println("Anime not found: " + id)
		NotFoundHandler(w, r)
		return
	}
	episode, err := getJsonEpisode(datas.Data, ep)
	if err != nil {
		log.Println("Episode not found: " + ep)
		NotFoundHandler(w, r)
		return
	}

	w.Header().Set("Content-Type", "video/mp4")
	go downloadEpisode(episode, cache)
	episodeName := episode.Vostfr.Title + "- Episode " + strconv.Itoa(episode.Vostfr.Num)
	videoIsReady, found := cache.Get(episodeName)
	if found {
		fmt.Println("Video found in cache")
		if videoIsReady.(string) == "downloading" {
			fmt.Println("Video is downloading")
			http.ServeFile(w, r, "public/encoding.mp4")
			return
		} else {
			fmt.Println("Video is ready")
			http.ServeFile(w, r, videoIsReady.(string))
			return
		}
	} else {
		fmt.Println("Video not found in cache")
		http.ServeFile(w, r, "public/encoding.mp4")
		return
	}
}

func downloadEpisode(episode EpisodeJson, cache *functions.Cache) (string, error) {
	// check if the video is already in the folder, and if it is, put it in the cache and return it
	episodeName := episode.Vostfr.Title + "- Episode " + strconv.Itoa(episode.Vostfr.Num)
	if _, err := os.Stat(episodeName + ".mp4"); err == nil {
		cache.Set(episodeName, episodeName+".mp4")
		return episodeName + ".mp4", nil
	}
	value, found := cache.Get(episodeName)
	if found {
		if value.(string) == "downloading" {
			return "", nil
		} else {
			return value.(string), nil
		}
	}
	fmt.Println("Downloading " + episodeName)
	cmd := exec.Command("ffmpeg", "-i", episode.Vostfr.VideoUri, "-c", "copy", "-bsf:a", "aac_adtstoasc", episodeName+".mp4")
	fmt.Println(cmd.String())
	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Println("Error creating stderr pipe:", err)
		return "", err
	}

	if err := cmd.Start(); err != nil {
		fmt.Println("Error starting ffmpeg:", err)
		return "", err
	}

	scanner := bufio.NewScanner(stderr)
	go cache.Set(episodeName, "downloading")
	for scanner.Scan() {
		m := scanner.Text()
		log.Println(m)
		if strings.Contains(m, "time=") {
			go cache.Set(episodeName, episodeName+".mp4")
		}
	}
	if err := cmd.Wait(); err != nil {
		return "", err
	}
	return episode.Vostfr.Title + ".mp4", nil

}

func getJsonEpisode(anime Fiche, episodeId string) (EpisodeJson, error) {
	_, err := getEpisode(anime, episodeId)
	if err != nil {
		log.Println("Episode not found: " + episodeId)
		return EpisodeJson{}, err
	}

	datas, err := client.Get("https://api.gazes.fr/anime/animes/" + strconv.Itoa(anime.Id) + "/" + episodeId)
	if err != nil {
		log.Println("Error while getting episode json: " + err.Error())
		return EpisodeJson{}, err
	}
	defer datas.Body.Close()
	var episode JsonResponseEpisode
	err = json.NewDecoder(datas.Body).Decode(&episode)
	if err != nil {
		log.Println("Error while decoding episode json: " + err.Error())
		return EpisodeJson{}, err
	}
	if !episode.Success {
		log.Println("Error while getting episode json: " + err.Error())
		return EpisodeJson{}, err
	}
	return episode.Data, nil
}

type JsonResponseEpisode struct {
	Success bool        `json:"success"`
	Data    EpisodeJson `json:"data"`
}

type EpisodeJson struct {
	Vostfr LangEpisode `json:"vostfr"`
	Vf     LangEpisode `json:"vf"`
}

type LangEpisode struct {
	VideoUri     string   `json:"videoUri"`
	VideoVtt     []string `json:"videoVtt"`
	VideoBaseUrl string   `json:"videoBaseUrl"`
	Time         string   `json:"time"`
	Episode      string   `json:"episode"`
	Num          int      `json:"num"`
	Title        string   `json:"title"`
	Url          string   `json:"url"`
	UrlImage     string   `json:"url_image"`
}
