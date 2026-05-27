// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

package domain

import "testing"

func TestMethodConstants_Values(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		want     string
	}{
		{"SendMessage", MethodSendMessage, "sendMessage"},
		{"SendFileByURL", MethodSendFileByURL, "sendFileByUrl"},
		{"SendLocation", MethodSendLocation, "sendLocation"},
		{"SendContact", MethodSendContact, "sendContact"},
		{"SendPoll", MethodSendPoll, "sendPoll"},
		{"ForwardMessages", MethodForwardMessages, "forwardMessages"},
		{"EditMessage", MethodEditMessage, "editMessage"},
		{"DeleteMessage", MethodDeleteMessage, "deleteMessage"},
		{"GetStateInstance", MethodGetStateInstance, "getStateInstance"},
		{"GetSettings", MethodGetSettings, "getSettings"},
		{"SetSettings", MethodSetSettings, "setSettings"},
		{"GetQR", MethodGetQR, "qr"},
		{"Reboot", MethodReboot, "reboot"},
		{"Logout", MethodLogout, "logout"},
		{"CheckWhatsapp", MethodCheckWhatsapp, "checkWhatsapp"},
		{"GetAvatar", MethodGetAvatar, "getAvatar"},
		{"GetContacts", MethodGetContacts, "getContacts"},
		{"GetContactInfo", MethodGetContactInfo, "getContactInfo"},
		{"ReceiveNotification", MethodReceiveNotification, "receiveNotification"},
		{"DeleteNotification", MethodDeleteNotification, "deleteNotification"},
		{"CreateGroup", MethodCreateGroup, "createGroup"},
		{"GetGroupData", MethodGetGroupData, "getGroupData"},
		{"AddGroupParticipant", MethodAddGroupParticipant, "addGroupParticipant"},
		{"RemoveGroupParticipant", MethodRemoveGroupParticipant, "removeGroupParticipant"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, tt.constant, tt.want)
			}
		})
	}
}

func TestMethodConstants_NotEmpty(t *testing.T) {
	methods := []string{
		MethodSendMessage, MethodSendFileByURL, MethodSendLocation,
		MethodSendContact, MethodSendPoll, MethodForwardMessages,
		MethodEditMessage, MethodDeleteMessage, MethodGetStateInstance,
		MethodGetSettings, MethodSetSettings, MethodGetQR,
		MethodReboot, MethodLogout, MethodCheckWhatsapp,
		MethodGetAvatar, MethodGetContacts, MethodGetContactInfo,
		MethodReceiveNotification, MethodDeleteNotification,
		MethodCreateGroup, MethodGetGroupData,
		MethodAddGroupParticipant, MethodRemoveGroupParticipant,
	}
	for _, m := range methods {
		if m == "" {
			t.Errorf("method constant should not be empty")
		}
	}
}

func TestMethodConstants_Unique(t *testing.T) {
	methods := []string{
		MethodSendMessage, MethodSendFileByURL, MethodSendLocation,
		MethodSendContact, MethodSendPoll, MethodForwardMessages,
		MethodEditMessage, MethodDeleteMessage, MethodGetStateInstance,
		MethodGetSettings, MethodSetSettings, MethodGetQR,
		MethodReboot, MethodLogout, MethodCheckWhatsapp,
		MethodGetAvatar, MethodGetContacts, MethodGetContactInfo,
		MethodReceiveNotification, MethodDeleteNotification,
		MethodCreateGroup, MethodGetGroupData,
		MethodAddGroupParticipant, MethodRemoveGroupParticipant,
	}
	seen := make(map[string]bool)
	for _, m := range methods {
		if seen[m] {
			t.Errorf("duplicate method constant: %q", m)
		}
		seen[m] = true
	}
}
