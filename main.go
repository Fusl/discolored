package main

import (
	"fmt"
	"image/color"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"regexp"

	"github.com/bwmarrin/discordgo"
	"golang.org/x/image/colornames"
)

func parseHexOrRGBToColor(s string) (color.RGBA, bool) {
	formats := []string{
		"%d %d %d",
		"%d,%d,%d",
		"#%02x%02x%02x",
		"%02x%02x%02x",
		"#%1x%1x%1x",
		"%1x%1x%1x",
	}
	var c color.RGBA
	for _, format := range formats {
		if _, err := fmt.Sscanf(s, format, &c.R, &c.G, &c.B); err == nil {
			return c, true
		}
	}
	return c, false
}

func parseColornameToColor(s string) (color.RGBA, bool) {
	reg, err := regexp.Compile("[^a-z]+")
	if err != nil {panic(err)}
	s = reg.ReplaceAllString(s, "")
	c, ok := colornames.Map[s]
	return c, ok
}

func parseMessageToColor(s string) (color.RGBA, bool) {
	if s != "" {
		parseFunctions := []func(s string) (c color.RGBA, ok bool){
			parseHexOrRGBToColor,
			parseColornameToColor,
		}
		for _, parseFunction := range parseFunctions {
			if c, ok := parseFunction(s); ok {
				return c, ok
			}
		}
	}
	return color.RGBA{}, false
}

func createUserRole(dc *discordgo.Session, guild string, user *discordgo.User) (role *discordgo.Role, err error) {
	role, err = dc.GuildRoleCreate(guild)
	if err != nil {
		return
	}
	role, err = dc.GuildRoleEdit(guild, role.ID, user.ID, role.Color, role.Hoist, role.Permissions, role.Mentionable)
	return
}

func RGBToRoleColor(c color.RGBA) int {
	return ((int(c.R) & 0x0ff) << 16) | ((int(c.G) & 0x0ff) << 8) | (int(c.B) & 0x0ff)
}

func updateUserRoleColor(dc *discordgo.Session, guild string, user *discordgo.User, role *discordgo.Role, c color.RGBA) (err error) {
	_, err = dc.GuildRoleEdit(guild, role.ID, role.Name, RGBToRoleColor(c), role.Hoist, role.Permissions, role.Mentionable)
	err = dc.GuildMemberRoleAdd(guild, user.ID, role.ID)
	return err
}

func upsertUserRoleColor(dc *discordgo.Session, guild string, user *discordgo.User, c color.RGBA) error {
	roles, err := dc.GuildRoles(guild)
	if err != nil {
		return err
	}
	var foundRole *discordgo.Role
	var roleFound bool
	for _, role := range roles {
		if role.Name == user.ID {
			foundRole = role
			roleFound = true
		}
	}
	if !roleFound {
		role, err := createUserRole(dc, guild, user)
		if err != nil {
			return err
		}
		foundRole = role
		roleFound = true
	}
	err = updateUserRoleColor(dc, guild, user, foundRole, c)
	return err
}

func main() {
	token := os.Getenv("TOKEN")
	channel := os.Getenv("CHANNEL")
	dc, err := discordgo.New("Bot " + token)
	if err != nil {
		panic(err)
	}
	dc.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.ChannelID != channel || m.Author.Bot {
			return
		}
		c, ok := parseMessageToColor(strings.ToLower(m.Content))
		if !ok {
			_ = dc.MessageReactionAdd(m.ChannelID, m.ID, "❌")
			return
		}
		err := upsertUserRoleColor(dc, m.GuildID, m.Author, c)
		if err != nil {
			_ = dc.MessageReactionAdd(m.ChannelID, m.ID, "❌")
			return
		}
		_ = dc.MessageReactionAdd(m.ChannelID, m.ID, "✅")
		return
	})
	dc.Identify.Intents = discordgo.IntentsGuildMessages
	err = dc.Open()
	if err != nil {
		panic(err)
	}
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
	_ = dc.Close()
}
