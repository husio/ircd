package ircd

import (
	"strings"
)

var commandHandlers = map[string]func(*Server, *Client, ...string){
	"NICK":    handleCommandNick,
	"USER":    handleCommandUser,
	"JOIN":    handleCommandJoin,
	"PING":    handleCommandPing,
	"QUIT":    handleCommandQuit,
	"PART":    handleCommandPart,
	"PRIVMSG": handleCommandPrivmsg,
}

const realm = "TODO-realm"

// Command: NICK
// Parameters: <nickname>
//
// NICK command is used to give user a nickname or change the existing one.
//
// Numeric Replies:
//
//  ERR_NONICKNAMEGIVEN             ERR_ERRONEUSNICKNAME
//  ERR_NICKNAMEINUSE               ERR_NICKCOLLISION
//  ERR_UNAVAILRESOURCE             ERR_RESTRICTED
//
func handleCommandNick(server *Server, client *Client, params ...string) {
	if len(params) == 0 {
		client.Send("%d %s :No nickname given.", ERR_NONICKNAMEGIVEN, client.Nick)
		return
	}
	// TODO(husio) check if nickname is available or restricted

	nick := params[0]

	server.mu.Lock()
	defer server.mu.Unlock()

	if _, ok := server.nicks[nick]; ok {
		client.Send("%d * %s :Nickname is already in use.", ERR_NICKNAMEINUSE, nick)
		return
	}
	if _, ok := server.channels[nick]; ok {
		client.Send("%d %s :Erroneus nickname.", ERR_ERRONEUSNICKNAME, nick)
		return
	}

	delete(server.nicks, client.Nick)
	server.nicks[nick] = client
	client.Nick = nick
	client.Send("%s!~%s@%s NICK :%s", client.Name, client.Nick, realm, client.Nick)
}

// Command: USER
// Parameters: <user> <mode> <unused> <realname>
//
// The USER command is used at the beginning of connection to specify
// the username, hostname and realname of a new user.
//
// The <mode> parameter should be a numeric, and can be used to
// automatically set user modes when registering with the server.  This
// parameter is a bitmask, with only 2 bits having any signification: if
// the bit 2 is set, the user mode 'w' will be set and if the bit 3 is
// set, the user mode 'i' will be set.  (See Section 3.1.5 "User
// Modes").
//
// The <realname> may contain space characters.
//
// Numeric Replies:
//
//  ERR_NEEDMOREPARAMS              ERR_ALREADYREGISTRED
//
func handleCommandUser(server *Server, client *Client, params ...string) {
	if len(params) != 4 {
		client.Send("%d %s :Need more params", ERR_NEEDMOREPARAMS, client.Nick)
		return
	}
	client.Name = params[0]

	client.Send("NOTICE AUTH :*** You connected on port 6667")
	// "Welcome to the Internet Relay Network <nick>!<user>@<host>"
	client.Send("%d %s :Welcome to IRCD", RPL_WELCOME, client.Nick)
	// "Your host is <servername>, running version <ver>"
	client.Send("%d %s :Your host is %s, running version %s", RPL_YOURHOST, client.Nick, client.c.RemoteAddr(), VERSION)
	// "This server was created <date>"
	client.Send("%d %s :This server was created %s", RPL_CREATED, client.Nick, client.server.created)
	// "<servername> <version> <available user modes> <available channel modes>"
	client.Send("%d %s :Welcome to IRCD", RPL_MYINFO, client.Nick)
	client.Send("%d %s :- %s Message of the day - ", RPL_MOTDSTART, "IRCD", client.Nick)
	client.Send("%d %s :- work in progress ", RPL_MOTD, client.Nick)
	client.Send("%d %s :End of /MOTD command", RPL_ENDOFMOTD, client.Nick)
}

func handleCommandPing(server *Server, client *Client, params ...string) {
}

// Command: JOIN
// Parameters: ( <channel> *( "," <channel> ) [ <key> *( "," <key> ) ] )
//             / "0"
//
// The JOIN command is used by a user to request to start listening to
// the specific channel.  Servers MUST be able to parse arguments in the
// form of a list of target, but SHOULD NOT use lists when sending JOIN
// messages to clients.
//
// Once a user has joined a channel, he receives information about
// all commands his server receives affecting the channel.  This
// includes JOIN, MODE, KICK, PART, QUIT and of course PRIVMSG/NOTICE.
// This allows channel members to keep track of the other channel
// members, as well as channel modes.
//
// If a JOIN is successful, the user receives a JOIN message as
// confirmation and is then sent the channel's topic (using RPL_TOPIC) and
// the list of users who are on the channel (using RPL_NAMREPLY), which
// MUST include the user joining.
//
// Note that this message accepts a special argument ("0"), which is
// a special request to leave all channels the user is currently a member
// of.  The server will process this message as if the user had sent
// a PART command (See Section 3.2.2) for each channel he is a member
// of.
//
// Numeric Replies:
//
//   ERR_NEEDMOREPARAMS              ERR_BANNEDFROMCHAN
//   ERR_INVITEONLYCHAN              ERR_BADCHANNELKEY
//   ERR_CHANNELISFULL               ERR_BADCHANMASK
//   ERR_NOSUCHCHANNEL               ERR_TOOMANYCHANNELS
//   ERR_TOOMANYTARGETS              ERR_UNAVAILRESOURCE
//   RPL_TOPIC
func handleCommandJoin(server *Server, client *Client, params ...string) {
	if len(params) == 0 {
		client.Send("%d %s :Not enough params", client.Nick)
		return
	}
	name := params[0]
	if name[0] != '#' {
		name = "#" + name
	}

	server.mu.Lock()
	defer server.mu.Unlock()

	channel, ok := server.channels[name]
	if !ok {
		channel = newChannel(name)
		server.channels[name] = channel
	}
	channel.clients[client.id] = client

	client.Send(":%s!~%s@%s JOIN :%s", client.Nick, client.Nick, realm, name)
	client.Send("%d %s %s :%s", RPL_TOPIC, client.Nick, name, name)

	client.channels[name] = channel

	names := make([]string, 0, len(channel.clients))
	for _, c := range channel.clients {
		names = append(names, c.Nick)
	}
	nicks := strings.Join(names, " ")
	for _, c := range channel.clients {
		c.Send("%d %s = %s :%s", RPL_NAMREPLY, c.Nick, name, nicks)
		c.Send("%d %s %s :End of NAMES list", RPL_ENDOFNAMES, c.Nick, name)
	}
}

func handleCommandPrivmsg(server *Server, client *Client, params ...string) {
}

func handleCommandQuit(server *Server, client *Client, params ...string) {
	client.Send("QUIT :Quit %s", strings.Join(params, " "))
	client.Close()

	server.mu.Lock()
	// TODO(husio) remove from all channels
	delete(server.clients, client.id)
	delete(server.nicks, client.Nick)
	server.mu.Unlock()
}

func handleCommandPart(server *Server, client *Client, params ...string) {
}
