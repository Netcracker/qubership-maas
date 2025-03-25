package utils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	_assert "github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
	"time"
)

type void struct{}

var member void

func TestUtils_CompactUuid(t *testing.T) {
	assert := _assert.New(t)

	// there is no Set in golang, so map keys represent set of unique string ids, while map values are ignored
	setOfIds := make(map[string]int, 1000)
	assert.Empty(setOfIds)

	for i := 0; i < 1000; i++ {
		newId := CompactUuid()
		_, found := setOfIds[newId]
		assert.False(found)
		setOfIds[newId] = 1
		assert.NotContains(newId, "-")
	}
	assert.Len(setOfIds, 1000)
}

func TestUtils_SlicesAreEqualInt32(t *testing.T) {
	assert := _assert.New(t)

	slice1 := []int32{0, 1, 2, 3, 4, 5}
	slice2 := []int32{0, 2, 1, 3, 4, 5}
	assert.False(SlicesAreEqualInt32(slice1, slice2))

	slice1 = []int32{0, 1, 2, 3, 4, 5}
	slice2 = []int32{0, 1, 2, 3, 4, 5}
	assert.True(SlicesAreEqualInt32(slice1, slice2))

	slice1 = []int32{0, 1, 2}
	slice2 = []int32{0, 1, 2, 3, 4, 5}
	assert.False(SlicesAreEqualInt32(slice1, slice2))

	slice1 = []int32{}
	slice2 = []int32{0, 1, 2, 3, 4, 5}
	assert.False(SlicesAreEqualInt32(slice1, slice2))

	slice1 = []int32{0, 1, 2, 3, 4, 5}
	slice2 = []int32{0, 1, 2}
	assert.False(SlicesAreEqualInt32(slice1, slice2))

	slice1 = []int32{0, 1, 2, 3, 4, 5}
	slice2 = []int32{}
	assert.False(SlicesAreEqualInt32(slice1, slice2))

	slice1 = []int32{}
	slice2 = []int32{}
	assert.True(SlicesAreEqualInt32(slice1, slice2))
}

func TestUtils_StringMapsAreEqual(t *testing.T) {
	assert := _assert.New(t)

	str1 := "str1"
	str2 := "str2"
	str2copy := "str2"
	var map1, map2 map[string]*string

	map1 = map[string]*string{"key0": nil, "key1": &str1, "key2": &str2}
	map2 = map[string]*string{"key0": nil, "key1": &str1, "key2": &str2}
	assert.True(StringMapsAreEqual(map1, map2))

	map1 = map[string]*string{"key0": nil, "key1": &str1, "key2": &str2}
	map2 = map[string]*string{"key0": nil, "key2": &str2, "key1": &str1}
	assert.True(StringMapsAreEqual(map1, map2))

	map1 = map[string]*string{"key0": nil, "key2": &str2, "key1": &str1}
	map2 = map[string]*string{"key0": nil, "key1": &str1, "key2": &str2}
	assert.True(StringMapsAreEqual(map1, map2))

	map1 = map[string]*string{"key0": nil, "key1": &str1, "key2": &str1}
	map2 = map[string]*string{"key0": nil, "key1": &str1, "key2": &str2}
	assert.False(StringMapsAreEqual(map1, map2))

	map1 = map[string]*string{"key0": nil, "key1": &str1, "key2": &str2}
	map2 = map[string]*string{"key0": nil, "key1": &str1}
	assert.False(StringMapsAreEqual(map1, map2))

	map1 = map[string]*string{"key0": nil, "key1": &str1}
	map2 = map[string]*string{"key0": nil, "key1": &str1, "key2": &str2}
	assert.False(StringMapsAreEqual(map1, map2))

	map1 = map[string]*string{}
	map2 = map[string]*string{"key0": nil, "key1": &str1, "key2": &str2}
	assert.False(StringMapsAreEqual(map1, map2))

	map1 = map[string]*string{"key0": nil, "key1": &str1, "key2": &str2}
	map2 = map[string]*string{}
	assert.False(StringMapsAreEqual(map1, map2))

	map1 = map[string]*string{"key0": nil, "key1": &str1, "key2": &str2}
	map2 = map[string]*string{"key0": nil, "key1": &str1, "key2": &str2copy}
	assert.True(StringMapsAreEqual(map1, map2))
}

func TestUtils_CheckChangesInOriginalSet(t *testing.T) {
	assert := _assert.New(t)

	str1 := "str1"
	str2 := "str2"
	str2copy := "str2"
	var orig, req map[string]*string

	orig = map[string]*string{"key0": nil, "key1": &str1, "key2": &str2}
	req = map[string]*string{"key0": nil, "key1": &str1, "key2": &str2}
	assert.True(CheckForChangesByRequest(orig, req))

	orig = map[string]*string{"key0": nil, "key1": &str1, "key2": &str2}
	req = map[string]*string{"key0": nil, "key2": &str2, "key1": &str1}
	assert.True(CheckForChangesByRequest(orig, req))

	orig = map[string]*string{"key0": nil, "key2": &str2, "key1": &str1}
	req = map[string]*string{"key0": nil, "key1": &str1, "key2": &str2}
	assert.True(CheckForChangesByRequest(orig, req))

	orig = map[string]*string{"key0": nil, "key1": &str1, "key2": &str1}
	req = map[string]*string{"key0": nil, "key1": &str1, "key2": &str2}
	assert.False(CheckForChangesByRequest(orig, req))

	orig = map[string]*string{"key0": nil, "key1": &str1, "key2": &str2}
	req = map[string]*string{"key0": nil, "key1": &str1}
	assert.True(CheckForChangesByRequest(orig, req))

	orig = map[string]*string{"key0": nil, "key1": &str1}
	req = map[string]*string{"key0": nil, "key1": &str1, "key2": &str2}
	assert.False(CheckForChangesByRequest(orig, req))

	orig = map[string]*string{}
	req = map[string]*string{"key0": nil, "key1": &str1, "key2": &str2}
	assert.False(CheckForChangesByRequest(orig, req))

	orig = map[string]*string{"key0": nil, "key1": &str1, "key2": &str2}
	req = map[string]*string{}
	assert.True(CheckForChangesByRequest(orig, req))

	orig = map[string]*string{"key0": nil, "key1": &str1, "key2": &str2}
	req = map[string]*string{"key0": nil, "key1": &str1, "key2": &str2copy}
	assert.True(CheckForChangesByRequest(orig, req))
}

func TestNormalizeYaml1(t *testing.T) {
	assert := _assert.New(t)
	input := `  
---
abc: cde
--- 
foo: bar

`
	actual, err := NormalizeJsonOrYamlInput(input)
	assert.NoError(err)
	assert.Equal(`[{"abc":"cde"},{"foo":"bar"}]`, actual)
}

func TestNormalizeYaml2(t *testing.T) {
	assert := _assert.New(t)
	input := `  
abc: "cde ---"
`
	actual, err := NormalizeJsonOrYamlInput(input)
	assert.NoError(err)
	assert.Equal(`[{"abc":"cde ---"}]`, actual)
}

func TestNormalizeJson1(t *testing.T) {
	assert := _assert.New(t)
	input := `
{
	"abc": "cde ---"
}
`
	actual, err := NormalizeJsonOrYamlInput(input)
	assert.NoError(err)
	assert.Equal("[{\n\t\"abc\": \"cde ---\"\n}]", actual)
}

func TestNormalizeYamlNull(t *testing.T) {
	actual, err := NormalizeJsonOrYamlInput(`a: null`)
	_assert.NoError(t, err)
	_assert.Equal(t, `[{"a":null}]`, actual)
}

func TestNormalizeJson2(t *testing.T) {
	assert := _assert.New(t)
	input := `
[{
	"abc": "cde ---"
}]
`
	actual, err := NormalizeJsonOrYamlInput(input)
	assert.NoError(err)
	assert.Equal("[{\n\t\"abc\": \"cde ---\"\n}]", actual)
}

func TestNormalizeJson_IgnoreEmptySections(t *testing.T) {
	assert := _assert.New(t)
	input := `
# some text 
---
abc: cde

# some other comment 
# and comments
---
# and licence 
`
	actual, err := NormalizeJsonOrYamlInput(input)
	assert.NoError(err)
	assert.Equal("[{\"abc\":\"cde\"}]", actual)
}

type Credentials struct {
	UserName string
	Password string `fmt:"obfuscate"`
}

func (v Credentials) Format(state fmt.State, verb int32) {
	FormatterUtil(v, state, verb)
}

func TestFormatterUtil(t *testing.T) {
	assert := _assert.New(t)
	userName := "scott"
	password := "tiger"
	credentials := Credentials{UserName: userName, Password: password}
	obfuscateSymbols := "***"

	actualOutput := fmt.Sprintf("%s", credentials)
	expectedOutput := fmt.Sprintf("{%s %s}", userName, obfuscateSymbols)
	assert.Equal(expectedOutput, actualOutput)

	actualOutput = fmt.Sprintf("%v", credentials)
	expectedOutput = fmt.Sprintf("{%v %v}", userName, obfuscateSymbols)
	assert.Equal(expectedOutput, actualOutput)

	actualOutput = fmt.Sprintf("%+v", credentials)
	expectedOutput = fmt.Sprintf("{UserName:%v Password:%v}", userName, obfuscateSymbols)
	assert.Equal(expectedOutput, actualOutput)

	actualOutput = fmt.Sprintf("%T", credentials)
	expectedOutput = "utils.Credentials"
	assert.Equal(expectedOutput, actualOutput)

	actualOutput = fmt.Sprintf("%#v", credentials)
	expectedOutput = fmt.Sprintf("utils.Credentials{UserName:%s Password:%s}", userName, obfuscateSymbols)
	assert.Equal(expectedOutput, actualOutput)
}

func Test_cancelableSleep(t *testing.T) {
	assert := _assert.New(t)
	start := time.Now()
	CancelableSleep(context.Background(), 1*time.Second)

	if time.Now().Sub(start) < 1*time.Second {
		assert.Fail("sleep must be cancelled")
	}
}

func Test_cancelableSleepCancel(t *testing.T) {
	assert := _assert.New(t)
	ctx, cancel := context.WithCancel(context.Background())

	start := time.Now()
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	CancelableSleep(ctx, 1*time.Second)

	if time.Now().Sub(start) >= 1*time.Second {
		assert.Fail("sleep must be cancelled")
	}
}

func TestMatchPattern(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx     context.Context
		pattern string
		value   any
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{name: "nil", args: args{ctx: ctx, pattern: "", value: nil}, want: false},
		{name: "empty_value", args: args{ctx: ctx, pattern: "*", value: ""}, want: true},
		{name: "one_asterisk", args: args{ctx: ctx, pattern: "*", value: "some value"}, want: true},
		{name: "suffix_asterisk", args: args{ctx: ctx, pattern: "so*", value: "some value"}, want: true},
		{name: "prefix_asterisk", args: args{ctx: ctx, pattern: "*ue", value: "some value"}, want: true},
		{name: "middle_asterisk", args: args{ctx: ctx, pattern: "so*ue", value: "some value"}, want: true},
		{name: "suffix_asterisk_not_matched", args: args{ctx: ctx, pattern: "wrong*", value: "some value"}, want: false},
		{name: "prefix_asterisk_not_matched", args: args{ctx: ctx, pattern: "*wrong", value: "some value"}, want: false},
		{name: "middle_asterisk_not_matched", args: args{ctx: ctx, pattern: "wr*ng", value: "some value"}, want: false},
		{name: "one_qm", args: args{ctx: ctx, pattern: "?", value: "s"}, want: true},
		{name: "suffix_qm", args: args{ctx: ctx, pattern: "so?", value: "som"}, want: true},
		{name: "prefix_qm", args: args{ctx: ctx, pattern: "?ue", value: "lue"}, want: true},
		{name: "middle_qm", args: args{ctx: ctx, pattern: "so?ue", value: "solue"}, want: true},
		{name: "suffix_qm_not_matched", args: args{ctx: ctx, pattern: "wrong?", value: "some value"}, want: false},
		{name: "prefix_qm_not_matched", args: args{ctx: ctx, pattern: "?wrong", value: "some value"}, want: false},
		{name: "middle_qm_not_matched", args: args{ctx: ctx, pattern: "wr?ng", value: "some value"}, want: false},
		{name: "qm_and_asterisk", args: args{ctx: ctx, pattern: "?om*ue", value: "some value"}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_assert.Equalf(t, tt.want, MatchPattern(tt.args.ctx, tt.args.pattern, tt.args.value), "MatchPattern(%v, %v, %v)", tt.args.ctx, tt.args.pattern, tt.args.value)
		})
	}
}

func TestRandomSubSequence(t *testing.T) {
	type args struct {
		src    []int32
		length int
	}
	tests := []struct {
		name     string
		args     args
		wantSize int
		wantErr  _assert.ErrorAssertionFunc
	}{
		{name: "nil", args: args{src: nil, length: 42}, wantSize: 0, wantErr: _assert.Error},
		{name: "src<length", args: args{src: []int32{4, 2}, length: 42}, wantSize: 0, wantErr: _assert.Error},
		{name: "src==length", args: args{src: []int32{4, 2}, length: 2}, wantSize: 2, wantErr: _assert.NoError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RandomSubSequence(tt.args.src, tt.args.length)
			if !tt.wantErr(t, err, fmt.Sprintf("RandomSubSequence(%v, %v)", tt.args.src, tt.args.length)) {
				return
			}
			_assert.Equalf(t, tt.wantSize, len(got), "RandomSubSequence(%v, %v)", tt.args.src, tt.args.length)
		})
	}
}

func TestRandomSubSequence_unique(t *testing.T) {
	srcSize := 20
	subSeqSize := 10
	src := make([]int32, srcSize)
	for i, v := range rand.Perm(srcSize) {
		src[i] = int32(v)
	}

	sequence, err := RandomSubSequence(src, subSeqSize)
	_assert.NoError(t, err)
	set := make(map[int32]void)
	for _, v := range sequence {
		set[v] = member
	}
	log.Info("selected items: %v", sequence)
	_assert.Equal(t, subSeqSize, len(set))
}

func TestWaitGroupWithTimeout_Wait(t *testing.T) {
	var waitGroup WaitGroupWithTimeout

	waitGroup.Add(1)
	go func() {
		waitGroup.Done()
	}()
	_assert.True(t, waitGroup.Wait(50*time.Millisecond))

	waitGroup.Add(1)
	go func() {
		time.Sleep(100 * time.Millisecond)
		waitGroup.Done()
	}()
	_assert.False(t, waitGroup.Wait(50*time.Millisecond))
}

func TestJsonFieldsOrdering(t *testing.T) {
	var objmap *map[string]interface{}
	if err := json.Unmarshal([]byte(`{"z":1,"b":{"z":2,"a":1},"a":2}`), &objmap); err != nil {
		t.Fatal("Can't deserialize json string to objects")
	}

	if s, err := json.Marshal(&objmap); err != nil {
		t.Fatal("Can't serialize object to json string")
	} else {
		if string(s) != `{"a":2,"b":{"a":1,"z":2},"z":1}` {
			fmt.Println(string(s))
			t.Fatal("Json has unsorted keys!")
		}
	}
}

func Test_Iff(t *testing.T) {
	_assert.Equal(t, "a", Iff(true, "a", "b"))
	_assert.Equal(t, "b", Iff(false, "a", "b"))
}

func Test_SubstractStringsArray(t *testing.T) {
	_assert.Equal(t, []string{"b"}, SubstractStringsArray([]string{"a", "b", "c"}, []string{"a", "c"}))
}

func TestRootCause(t *testing.T) {
	var e1 = errors.New("oops")
	var e2 = fmt.Errorf("wrapping: %w", e1)
	var e3 = fmt.Errorf("another wrapping: %w", e2)

	_assert.Equal(t, e1, RootCause(e3))
}

func TestRootCause2(t *testing.T) {
	var e1 = errors.New("oops")
	_assert.Equal(t, e1, RootCause(e1))
}

func TestRunWithRetryValue1(t *testing.T) {
	ctx := context.Background()
	counter := 5
	result, err := RetryValue(ctx, 1*time.Second, 100*time.Microsecond, func(ctx context.Context) (string, error) {
		counter--
		if counter == 0 {
			return "ok", nil
		}
		return "", errors.New("error")
	})
	_assert.NoError(t, err)
	_assert.Equal(t, "ok", result)
	_assert.Equal(t, 0, counter)
}

func TestRunWithRetryValue_Timeout(t *testing.T) {
	ctx := context.Background()

	counter := 20
	cause := errors.New("some error")
	err := Retry(ctx, 1*time.Second, 200*time.Millisecond, func(ctx context.Context) error {
		counter--
		if counter == 0 {
			return nil
		}
		return cause
	})
	_assert.ErrorIs(t, err, cause)
	_assert.True(t, counter > 0)
}

func TestRunWithRetryValue3(t *testing.T) {
	ctx := context.Background()
	counter := 0
	err := Retry(ctx, 1100*time.Millisecond, 200*time.Millisecond, func(ctx context.Context) error {
		counter++
		return errors.New("error")
	})
	_assert.Error(t, err)
	_assert.Equal(t, counter, 6)
}

func TestRunWithRetryValue_FastFirstTry(t *testing.T) {
	ctx := context.Background()
	deadline := time.Now().Add(250 * time.Millisecond)
	err := Retry(ctx, 5*time.Second,
		2*time.Second, // setup long value for first retry
		func(ctx context.Context) error {
			return nil
		})
	_assert.NoError(t, err)
	_assert.True(t, deadline.After(time.Now()))
}
