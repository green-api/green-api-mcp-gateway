// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

package domain

// Green API method name constants.
const (
	MethodSendMessage         = "sendMessage"
	MethodSendFileByURL       = "sendFileByUrl"
	MethodSendLocation        = "sendLocation"
	MethodSendContact         = "sendContact"
	MethodSendPoll            = "sendPoll"
	MethodForwardMessages     = "forwardMessages"
	MethodEditMessage         = "editMessage"
	MethodDeleteMessage       = "deleteMessage"
	MethodGetStateInstance    = "getStateInstance"
	MethodGetSettings         = "getSettings"
	MethodSetSettings         = "setSettings"
	MethodGetQR               = "qr"
	MethodReboot              = "reboot"
	MethodLogout              = "logout"
	MethodCheckWhatsapp       = "checkWhatsapp"
	MethodGetAvatar           = "getAvatar"
	MethodGetContacts         = "getContacts"
	MethodGetContactInfo      = "getContactInfo"
	MethodReceiveNotification = "receiveNotification"
	MethodDeleteNotification  = "deleteNotification"

	// Group methods
	MethodCreateGroup            = "createGroup"
	MethodGetGroupData           = "getGroupData"
	MethodAddGroupParticipant    = "addGroupParticipant"
	MethodRemoveGroupParticipant = "removeGroupParticipant"
	MethodSetGroupAdmin          = "setGroupAdmin"
	MethodRemoveGroupAdmin       = "removeGroupAdmin"
	MethodLeaveGroup             = "leaveGroup"

	// Instance management
	MethodGetWaSettings        = "getWaSettings"
	MethodGetAuthorizationCode = "getAuthorizationCode"

	// History / reading
	MethodGetChatHistory       = "getChatHistory"
	MethodGetMessage           = "getMessage"
	MethodLastIncomingMessages = "lastIncomingMessages"
	MethodLastOutgoingMessages = "lastOutgoingMessages"
	MethodReadChat             = "readChat"

	// File upload
	MethodUploadFile       = "uploadFile"
	MethodSendFileByUpload = "sendFileByUpload"
)
