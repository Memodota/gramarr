package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tommy647/gramarr/internal/users"

	"github.com/tommy647/gramarr/internal/radarr"
	"github.com/tommy647/gramarr/internal/util"
	"gopkg.in/tucnak/telebot.v2"
	tb "gopkg.in/tucnak/telebot.v2"
)

func NewAddMovieConversation(e *Service) *AddMovieConversation {
	return &AddMovieConversation{env: e}
}

type AddMovieConversation struct {
	currentStep            func(*tb.Message)
	movieQuery             string
	movieResults           []radarr.Movie
	folderResults          []radarr.Folder
	selectedMovie          *radarr.Movie
	selectedQualityProfile *radarr.Profile
	selectedFolder         *radarr.Folder
	env                    *Service
}

func (c *AddMovieConversation) Run(m *tb.Message) {
	c.currentStep = c.AskMovie(m)
}

func (c *AddMovieConversation) Name() string {
	return "addmovie"
}

func (c *AddMovieConversation) CurrentStep() func(*telebot.Message) {
	return c.currentStep
}

func (c *AddMovieConversation) AskMovie(m *tb.Message) func(*telebot.Message) {
	user := users.User{} // @todo: fix
	util.Send(c.env.Bot, user, "What movie do you want to search for?")

	return func(m *tb.Message) {
		c.movieQuery = m.Text

		movies, err := c.env.Radarr.SearchMovies(c.movieQuery)
		c.movieResults = movies

		// Search Service Failed
		if err != nil {
			util.SendError(c.env.Bot, user, "Failed to search movies.")
			c.env.CM.StopConversation(c)
			return
		}

		// No Results
		if len(movies) == 0 {
			msg := fmt.Sprintf("No movie found with the title '%s'", util.EscapeMarkdown(c.movieQuery))
			util.Send(c.env.Bot, user, msg)
			c.env.CM.StopConversation(c)
			return
		}

		// Found some movies! Yay!
		var msg []string
		msg = append(msg, fmt.Sprintf("*Found %d movies:*", len(movies)))
		for i, movie := range movies {
			msg = append(msg, fmt.Sprintf("%d) %s", i+1, util.EscapeMarkdown(movie.String())))
		}
		util.Send(c.env.Bot, user, strings.Join(msg, "\n"))
		c.currentStep = c.AskPickMovie(m)
	}
}

func (c *AddMovieConversation) AskPickMovie(m *tb.Message) func(*telebot.Message) {
	user := users.User{} // @todo: fix
	// Send custom reply keyboard
	var options []string
	for _, movie := range c.movieResults {
		options = append(options, fmt.Sprintf("%s", movie))
	}
	options = append(options, "/cancel")
	util.SendKeyboardList(c.env.Bot, user, "Which one would you like to download?", options)

	return func(m *tb.Message) {

		// Set the selected movie
		for i, opt := range options {
			if m.Text == opt {
				c.selectedMovie = &c.movieResults[i]
				break
			}
		}

		// Not a valid movie selection
		if c.selectedMovie == nil {
			util.SendError(c.env.Bot, user, "Invalid selection.")
			c.currentStep = c.AskPickMovie(m)
			return
		}

		c.currentStep = c.AskPickMovieQuality(m)
	}
}

func (c *AddMovieConversation) AskPickMovieQuality(m *tb.Message) func(*telebot.Message) {
	user := users.User{} // @todo: fix
	profiles, err := c.env.Radarr.GetProfile("profile")

	// GetProfile Service Failed
	if err != nil {
		util.SendError(c.env.Bot, user, "Failed to get quality profiles.")
		c.env.CM.StopConversation(c)
		return nil
	}

	// Send custom reply keyboard
	var options []string
	for _, QualityProfile := range profiles {
		options = append(options, fmt.Sprintf("%v", QualityProfile.Name))
	}
	options = append(options, "/cancel")
	util.SendKeyboardList(c.env.Bot, user, "Which quality shall I look for?", options)

	return func(m *tb.Message) {
		// Set the selected option
		for i := range options {
			if m.Text == options[i] {
				c.selectedQualityProfile = &profiles[i]
				break
			}
		}

		// Not a valid selection
		if c.selectedQualityProfile == nil {
			util.SendError(c.env.Bot, user, "Invalid selection.")
			c.currentStep = c.AskPickMovieQuality(m)
			return
		}

		c.currentStep = c.AskFolder(m)
	}
}

func (c *AddMovieConversation) AskFolder(m *tb.Message) func(*telebot.Message) {
	user := users.User{} // @todo: fix
	folders, err := c.env.Radarr.GetFolders()
	c.folderResults = folders

	// GetFolders Service Failed
	if err != nil {
		util.SendError(c.env.Bot, user, "Failed to get folders.")
		c.env.CM.StopConversation(c)
		return nil
	}

	// No Results
	if len(folders) == 0 {
		util.SendError(c.env.Bot, user, "No destination folders found.")
		c.env.CM.StopConversation(c)
		return nil
	}

	// Found folders!

	// Send the results
	var msg []string
	msg = append(msg, fmt.Sprintf("*Found %d folders:*", len(folders)))
	for i, folder := range folders {
		msg = append(msg, fmt.Sprintf("%d) %s", i+1, util.EscapeMarkdown(filepath.Base(folder.Path))))
	}
	util.Send(c.env.Bot, user, strings.Join(msg, "\n"))

	// Send the custom reply keyboard
	var options []string
	for _, folder := range folders {
		options = append(options, fmt.Sprintf("%s", filepath.Base(folder.Path)))
	}
	options = append(options, "/cancel")
	util.SendKeyboardList(c.env.Bot, user, "Which folder should it download to?", options)

	return func(m *tb.Message) {
		// Set the selected folder
		for i, opt := range options {
			if m.Text == opt {
				c.selectedFolder = &c.folderResults[i]
				break
			}
		}

		// Not a valid folder selection
		if c.selectedMovie == nil {
			util.SendError(c.env.Bot, user, "Invalid selection.")
			c.currentStep = c.AskFolder(m)
			return
		}

		c.AddMovie(m)
	}
}

func (c *AddMovieConversation) AddMovie(m *tb.Message) {
	user := users.User{} // @todo: fix this, user in context
	_, err := c.env.Radarr.AddMovie(*c.selectedMovie, c.selectedQualityProfile.ID, c.selectedFolder.Path)

	// Failed to add movie
	if err != nil {
		util.SendError(c.env.Bot, user, "Failed to add movie.")
		c.env.CM.StopConversation(c)
		return
	}

	if c.selectedMovie.PosterURL != "" {
		photo := &tb.Photo{File: tb.FromURL(c.selectedMovie.PosterURL)}
		c.env.Bot.Send(user, photo)
	}

	// Notify User
	util.Send(c.env.Bot, user, "Movie has been added!")

	// Notify Admin
	adminMsg := fmt.Sprintf("%s added movie '%s'", user.DisplayName(), util.EscapeMarkdown(c.selectedMovie.String()))
	util.SendAdmin(c.env.Bot, c.env.Users.Admins(), adminMsg)

	c.env.CM.StopConversation(c)
}
