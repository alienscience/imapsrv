// Package mysqlstore holds an implementation of github.com/alienscience/imapsrv/auth - AuthStore, using MySQL
package mysqlstore

// TODO: implement all these functions for MySQL... but with which driver?
// or do we want two packages, one for each driver:
// go-sql-driver/mysql
// ziutek/mymysql

type MySQLAuthStore struct {
}

// Authenticate attempts to authenticate the given credentials
func (m *MySQLAuthStore) Authenticate(username, plainPassword string) (success bool, err error) {
	return false, nil
}

// CreateUser creates a user with the given username
func (m *MySQLAuthStore) CreateUser(username, plainPassword string) error {
	return nil
}

// ResetPassword resets the password for the given username
func (m *MySQLAuthStore) ResetPassword(username, plainPassword string) error {
	return nil
}

// ListUsers lists all information about the users
// TODO: this could be very neat for the sysadmin, but probably a lot of metadata
// 		 about users is desired, and not just the usernames.
func (m *MySQLAuthStore) ListUsers() (usernames []string, err error) {
	return []string{}, nil
}

// DeleteUser removes the username from the database entirely
func (m *MySQLAuthStore) DeleteUser(username string) error {
	return nil
}
