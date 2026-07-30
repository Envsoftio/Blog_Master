package redis

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

var ErrNil = errors.New("redis nil response")

type Config struct {
	Addr     string
	Password string
	Timeout  time.Duration
}

type Client struct {
	addr     string
	password string
	timeout  time.Duration
}

func New(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 150 * time.Millisecond
	}
	return &Client{
		addr:     strings.TrimSpace(cfg.Addr),
		password: cfg.Password,
		timeout:  timeout,
	}
}

func (c *Client) Ping(ctx context.Context) error {
	response, err := c.command(ctx, []byte("PING"))
	if err != nil {
		return err
	}
	if strings.EqualFold(string(response), "PONG") {
		return nil
	}
	return fmt.Errorf("unexpected redis PING response %q", response)
}

func (c *Client) Get(ctx context.Context, key string) ([]byte, bool, error) {
	response, err := c.command(ctx, []byte("GET"), []byte(key))
	if err != nil {
		if errors.Is(err, ErrNil) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return response, true, nil
}

func (c *Client) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		return errors.New("redis SET requires a positive ttl")
	}
	seconds := strconv.FormatInt(int64(ttl.Round(time.Second)/time.Second), 10)
	if seconds == "0" {
		seconds = "1"
	}
	response, err := c.command(ctx,
		[]byte("SET"),
		[]byte(key),
		value,
		[]byte("EX"),
		[]byte(seconds),
	)
	if err != nil {
		return err
	}
	if strings.EqualFold(string(response), "OK") {
		return nil
	}
	return fmt.Errorf("unexpected redis SET response %q", response)
}

func (c *Client) command(ctx context.Context, args ...[]byte) ([]byte, error) {
	if c.addr == "" {
		return nil, errors.New("redis address is empty")
	}
	dialer := net.Dialer{Timeout: c.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", c.addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	deadline := time.Now().Add(c.timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	_ = conn.SetDeadline(deadline)

	reader := bufio.NewReader(conn)
	if c.password != "" {
		if err := writeCommand(conn, [][]byte{[]byte("AUTH"), []byte(c.password)}); err != nil {
			return nil, err
		}
		response, err := readResponse(reader)
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(string(response), "OK") {
			return nil, fmt.Errorf("unexpected redis AUTH response %q", response)
		}
	}
	if err := writeCommand(conn, args); err != nil {
		return nil, err
	}
	return readResponse(reader)
}

func writeCommand(writer io.Writer, args [][]byte) error {
	var buffer bytes.Buffer
	buffer.WriteByte('*')
	buffer.WriteString(strconv.Itoa(len(args)))
	buffer.WriteString("\r\n")
	for _, arg := range args {
		buffer.WriteByte('$')
		buffer.WriteString(strconv.Itoa(len(arg)))
		buffer.WriteString("\r\n")
		buffer.Write(arg)
		buffer.WriteString("\r\n")
	}
	_, err := writer.Write(buffer.Bytes())
	return err
}

func readResponse(reader *bufio.Reader) ([]byte, error) {
	prefix, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	switch prefix {
	case '+':
		return []byte(line), nil
	case '-':
		return nil, errors.New(line)
	case ':':
		return []byte(line), nil
	case '$':
		length, err := strconv.Atoi(line)
		if err != nil {
			return nil, err
		}
		if length < 0 {
			return nil, ErrNil
		}
		payload := make([]byte, length+2)
		if _, err := io.ReadFull(reader, payload); err != nil {
			return nil, err
		}
		if payload[length] != '\r' || payload[length+1] != '\n' {
			return nil, errors.New("malformed redis bulk string terminator")
		}
		return payload[:length], nil
	default:
		return nil, fmt.Errorf("unsupported redis response prefix %q", prefix)
	}
}
