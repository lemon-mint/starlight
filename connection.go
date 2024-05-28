package starlight

type connection struct {
	protocol   string
	buffer     []byte
	blockWrite bool
	close      bool
}

func (c *connection) Protocol() string {
	return c.protocol
}

func (c *connection) Close() {

}

type connectionTable struct {
}
