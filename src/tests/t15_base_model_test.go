// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package tests

import (
	"fmt"
	"testing"

	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/q"
	. "github.com/smartystreets/goconvey/convey"
)

func TestBaseModelMethods(t *testing.T) {
	Convey("Testing base model methods", t, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			userJane := h.User().Search(env, q.User().Email().Equals("jane.smith@example.com"))
			Convey("Copy", func() {
				newProfile := userJane.Profile().Copy(&h.ProfileData{})
				userJane.Write(h.User().NewData().SetPassword("Jane's Password"))
				userJaneCopy := userJane.Copy(h.User().NewData().
					SetName("Jane's Copy").
					SetEmail2("js@example.com").
					SetProfile(newProfile))
				So(userJaneCopy.Name(), ShouldEqual, "Jane's Copy")
				So(userJaneCopy.Email(), ShouldEqual, "jane.smith@example.com")
				So(userJaneCopy.Email2(), ShouldEqual, "js@example.com")
				So(userJaneCopy.Password(), ShouldBeBlank)
				So(userJaneCopy.Age(), ShouldEqual, 24)
				So(userJaneCopy.Nums(), ShouldEqual, 2)
				So(userJaneCopy.Posts().Len(), ShouldEqual, 2)
			})
			Convey("Sorted", func() {
				for i := 0; i < 20; i++ {
					h.Post().Create(env, h.Post().NewData().
						SetTitle(fmt.Sprintf("Post no %02d", (24-i)%20)).
						SetUser(userJane))
				}
				posts := h.Post().Search(env, q.Post().Title().Contains("Post no")).OrderBy("ID")
				for i, post := range posts.Records() {
					So(post.Title(), ShouldEqual, fmt.Sprintf("Post no %02d", (24-i)%20))
				}

				sortedPosts := posts.Sorted(func(rs1, rs2 h.PostSet) bool {
					return rs1.Title() < rs2.Title()
				}).Records()
				So(sortedPosts, ShouldHaveLength, 20)
				for i, post := range sortedPosts {
					So(post.Title(), ShouldEqual, fmt.Sprintf("Post no %02d", i))
				}
			})
			Convey("Filtered", func() {
				for i := 0; i < 20; i++ {
					h.Post().Create(env, h.Post().NewData().
						SetTitle(fmt.Sprintf("Post no %02d", i)).
						SetUser(userJane))
				}
				posts := h.Post().Search(env, q.Post().Title().Contains("Post no"))

				evenPosts := posts.Filtered(func(rs h.PostSet) bool {
					var num int
					fmt.Sscanf(rs.Title(), "Post no %02d", &num)
					if num%2 == 0 {
						return true
					}
					return false
				}).Records()
				So(evenPosts, ShouldHaveLength, 10)
				for i := 0; i < 10; i++ {
					So(evenPosts[i].Title(), ShouldEqual, fmt.Sprintf("Post no %02d", 2*i))
				}
			})
		}), ShouldBeNil)
	})
}
