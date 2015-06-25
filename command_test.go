package imapsrv

import "testing"
import "fmt"

func setupTest() (*Server, *session) {
	m := &TestMailstore{}
	s := NewServer(
		Store(m),
	)
	//s.Start()
	sess := createSession("1", s.config, s, nil, nil) // TODO: listener and net.Conn
	return s, sess
}

// A test mailstore used for unit testing
type TestMailstore struct {
}

// Get mailbox information
func (m *TestMailstore) GetMailbox(path []string) (*Mailbox, error) {
	return &Mailbox{
		Name: "inbox",
		Id:   1,
	}, nil
}

// Get a list of mailboxes at the given path
func (m *TestMailstore) GetMailboxes(path []string) ([]*Mailbox, error) {
	if len(path) == 0 {
		// Root
		return []*Mailbox{
			{
				Name: "inbox",
				Path: []string{"inbox"},
				Id:   1,
			},
			{
				Name: "spam",
				Path: []string{"spam"},
				Id:   2,
			},
		}, nil
	} else if len(path) == 1 && path[0] == "inbox" {
		return []*Mailbox{
			{
				Name: "starred",
				Path: []string{"inbox", "stared"},
				Id:   3,
			},
		}, nil
	} else {
		return []*Mailbox{}, nil
	}
}

// Get the sequence number of the first unseen message
func (m *TestMailstore) FirstUnseen(mbox int64) (int64, error) {
	return 4, nil
}

// Get the total number of messages in an IMAP mailbox
func (m *TestMailstore) TotalMessages(mbox int64) (int64, error) {
	return 8, nil
}

// Get the total number of unread messages in an IMAP mailbox
func (m *TestMailstore) RecentMessages(mbox int64) (int64, error) {
	return 4, nil
}

// Get the next available uid in an IMAP mailbox
func (m *TestMailstore) NextUid(mbox int64) (int64, error) {
	return 9, nil
}

func TestCapabilityCommand(t *testing.T) {
	_, session := setupTest()
	cap := &capability{tag: "A00001"}
	resp := cap.execute(session)
	// TODO: STARTTLS shouldn't always be available? (i.e. after using STARTTLS)
	if (resp.tag != "A00001") || (resp.message != "CAPABILITY completed") || (resp.untagged[0] != "CAPABILITY IMAP4rev1 STARTTLS") {
		t.Error("Capability Failed - unexpected response.")
		fmt.Println(resp)
	}
}

func TestLogoutCommand(t *testing.T) {
	_, session := setupTest()
	log := &logout{tag: "A00004"}
	resp := log.execute(session)
	if (resp.tag != "A00004") || (resp.message != "LOGOUT completed") || (resp.untagged[0] != "BYE IMAP4rev1 Server logging out") {
		t.Error("Logout Failed - unexpected response.")
		fmt.Println(resp)
	}
}
