package udf

import (
	. "github.com/smartystreets/goconvey/convey"
	"io"
	"io/ioutil"
	"pfi/sensorbee/sensorbee/core"
	"testing"
)

func TestInMemoryUDSStorage(t *testing.T) {
	Convey("Given an in memory UDSStorage with a saved state", t, func() {
		s := NewInMemoryUDSStorage()

		w, err := s.Save("test_topology", "state1")
		So(err, ShouldBeNil)
		_, err = io.WriteString(w, "hoge")
		So(err, ShouldBeNil)
		So(w.Commit(), ShouldBeNil)

		Convey("When loading the state", func() {
			r, err := s.Load("test_topology", "state1")
			So(err, ShouldBeNil)
			data, err := ioutil.ReadAll(r)
			So(err, ShouldBeNil)

			Convey("Then it should have the right content", func() {
				So(string(data), ShouldEqual, "hoge")
			})
		})

		Convey("When listing states", func() {
			l, err := s.List()
			So(err, ShouldBeNil)

			Convey("Then it should have the state", func() {
				So(len(l), ShouldEqual, 1)
				So(len(l["test_topology"]), ShouldEqual, 1)
				So(l["test_topology"][0], ShouldEqual, "state1")
			})
		})

		Convey("When loading the state with a wrong topology name", func() {
			_, err := s.Load("test_topology2", "state1")

			Convey("Then it should fail", func() {
				So(core.IsNotExist(err), ShouldBeTrue)
			})
		})

		Convey("When loading the state with a wrong state name", func() {
			_, err := s.Load("test_topology", "state2")

			Convey("Then it should fail", func() {
				So(core.IsNotExist(err), ShouldBeTrue)
			})
		})

		Convey("When overwriting the state", func() {
			w, err := s.Save("test_topology", "state1")
			So(err, ShouldBeNil)
			_, err = io.WriteString(w, "fuga")
			So(err, ShouldBeNil)

			Convey("Then the content should be updated with Commit", func() {
				So(w.Commit(), ShouldBeNil)
				r, err := s.Load("test_topology", "state1")
				So(err, ShouldBeNil)
				data, err := ioutil.ReadAll(r)
				So(err, ShouldBeNil)
				So(string(data), ShouldEqual, "fuga")
			})

			Convey("Then the content should not be updated with Abort", func() {
				So(w.Abort(), ShouldBeNil)
				r, err := s.Load("test_topology", "state1")
				So(err, ShouldBeNil)
				data, err := ioutil.ReadAll(r)
				So(err, ShouldBeNil)
				So(string(data), ShouldEqual, "hoge")
			})
		})

		Convey("When saving another state", func() {
			w, err := s.Save("test_topology", "state2")
			So(err, ShouldBeNil)
			_, err = io.WriteString(w, "fuga")
			So(err, ShouldBeNil)
			So(w.Commit(), ShouldBeNil)

			Convey("Then it should be able to be loaded", func() {
				r, err := s.Load("test_topology", "state2")
				So(err, ShouldBeNil)
				data, err := ioutil.ReadAll(r)
				So(err, ShouldBeNil)
				So(string(data), ShouldEqual, "fuga")
			})

			Convey("Then state1 should also be able to be loaded", func() {
				r, err := s.Load("test_topology", "state1")
				So(err, ShouldBeNil)
				data, err := ioutil.ReadAll(r)
				So(err, ShouldBeNil)
				So(string(data), ShouldEqual, "hoge")
			})

			Convey("And listing states", func() {
				l, err := s.List()
				So(err, ShouldBeNil)

				Convey("Then it should have the states", func() {
					So(len(l), ShouldEqual, 1)
					So(len(l["test_topology"]), ShouldEqual, 2)
					So(l["test_topology"], ShouldContain, "state1")
					So(l["test_topology"], ShouldContain, "state2")
				})
			})
		})

		Convey("When writing to a committed writer", func() {
			_, err := io.WriteString(w, "hogehoge")

			Convey("Then it should fail", func() {
				So(err, ShouldNotBeNil)
			})
		})
	})
}