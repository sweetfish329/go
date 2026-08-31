package repository

// rowScanner abstracts *sql.Row and *sql.Rows for unified scanning
type rowScanner interface {
	Scan(dest ...interface{}) error
}
