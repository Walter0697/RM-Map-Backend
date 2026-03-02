package service

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"mapmarker/backend/config"
	"net"
	"strconv"
	"strings"
	"time"
)

type redisClient interface {
	Ping() error
	Get(key string) (string, bool, error)
	SetEX(key string, value string, ttl time.Duration) error
	Del(key string) error
}

type rawRedisClient struct{}

func newRawRedisClient() redisClient {
	return &rawRedisClient{}
}

func (c *rawRedisClient) Ping() error {
	reply, err := c.do("PING")
	if err != nil {
		return err
	}
	text, ok := reply.(string)
	if !ok || strings.ToUpper(text) != "PONG" {
		return fmt.Errorf("unexpected PING reply: %v", reply)
	}
	return nil
}

func (c *rawRedisClient) Get(key string) (string, bool, error) {
	reply, err := c.do("GET", key)
	if err != nil {
		return "", false, err
	}
	if reply == nil {
		return "", false, nil
	}
	text, ok := reply.(string)
	if !ok {
		return "", false, fmt.Errorf("unexpected GET reply type %T", reply)
	}
	return text, true, nil
}

func (c *rawRedisClient) SetEX(key string, value string, ttl time.Duration) error {
	seconds := int(ttl / time.Second)
	if seconds <= 0 {
		seconds = 1
	}
	reply, err := c.do("SETEX", key, strconv.Itoa(seconds), value)
	if err != nil {
		return err
	}
	text, ok := reply.(string)
	if !ok || strings.ToUpper(text) != "OK" {
		return fmt.Errorf("unexpected SETEX reply: %v", reply)
	}
	return nil
}

func (c *rawRedisClient) Del(key string) error {
	_, err := c.do("DEL", key)
	return err
}

func (c *rawRedisClient) do(args ...string) (interface{}, error) {
	address := net.JoinHostPort(strings.TrimSpace(config.Data.Redis.Host), strings.TrimSpace(config.Data.Redis.Port))
	dialTimeout := time.Duration(config.Data.AuthState.RedisDialTimeoutMS) * time.Millisecond
	readTimeout := time.Duration(config.Data.AuthState.RedisReadTimeoutMS) * time.Millisecond
	writeTimeout := time.Duration(config.Data.AuthState.RedisWriteTimeoutMS) * time.Millisecond

	conn, err := net.DialTimeout("tcp", address, dialTimeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
		return nil, err
	}
	if err := conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		return nil, err
	}

	reader := bufio.NewReader(conn)

	if strings.TrimSpace(config.Data.Redis.Password) != "" {
		if _, err := writeRESPCommand(conn, "AUTH", config.Data.Redis.Password); err != nil {
			return nil, err
		}
		if _, err := readRESP(reader); err != nil {
			return nil, err
		}
	}

	if config.Data.Redis.DB > 0 {
		if _, err := writeRESPCommand(conn, "SELECT", strconv.Itoa(config.Data.Redis.DB)); err != nil {
			return nil, err
		}
		if _, err := readRESP(reader); err != nil {
			return nil, err
		}
	}

	if _, err := writeRESPCommand(conn, args...); err != nil {
		return nil, err
	}

	reply, err := readRESP(reader)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

func writeRESPCommand(w io.Writer, args ...string) (int, error) {
	var builder strings.Builder
	builder.WriteString("*")
	builder.WriteString(strconv.Itoa(len(args)))
	builder.WriteString("\r\n")
	for _, arg := range args {
		builder.WriteString("$")
		builder.WriteString(strconv.Itoa(len(arg)))
		builder.WriteString("\r\n")
		builder.WriteString(arg)
		builder.WriteString("\r\n")
	}
	return io.WriteString(w, builder.String())
}

func readRESP(reader *bufio.Reader) (interface{}, error) {
	prefix, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	switch prefix {
	case '+':
		text, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		return strings.TrimSuffix(strings.TrimSuffix(text, "\n"), "\r"), nil
	case '-':
		text, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		return nil, errors.New(strings.TrimSuffix(strings.TrimSuffix(text, "\n"), "\r"))
	case ':':
		text, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		value, convErr := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
		if convErr != nil {
			return nil, convErr
		}
		return value, nil
	case '$':
		text, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		size, convErr := strconv.Atoi(strings.TrimSpace(text))
		if convErr != nil {
			return nil, convErr
		}
		if size < 0 {
			return nil, nil
		}
		buffer := make([]byte, size+2)
		if _, err := io.ReadFull(reader, buffer); err != nil {
			return nil, err
		}
		return string(buffer[:size]), nil
	default:
		return nil, fmt.Errorf("unsupported redis reply prefix %q", string(prefix))
	}
}
