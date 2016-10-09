package tcp

func (c *Conn) Read(b []byte) (n int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// TODO(joshlf): actually check to see if there's data
	nodata := true
	for nodata {
		c.readCond.Wait()
	}

	panic("not implemented")
}

func (c *Conn) Write(b []byte) (n int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// TODO(joshlf): actually check to see if there's space
	bufferspace := false
	for !bufferspace {
		c.writeCond.Wait()
	}

	panic("not implemented")
}
