/*
**	CommandIDs_cl.h		Clanlord
**
**	common header file for menu command IDs
**
**	Copyright 2023 Delta Tao Software, Inc.
** 
**	Licensed under the Apache License, Version 2.0 (the "License");
**	you may not use this file except in compliance with the License.
**	You may obtain a copy of the License at
** 
**     https://www.apache.org/licenses/LICENSE-2.0
** 
**	Unless required by applicable law or agreed to in writing, software
**	distributed under the License is distributed on an "AS IS" BASIS,
**	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
**	See the License for the specific language governing permissions and
**	imitations under the License.
*/

#ifndef COMMAND_IDS_CL_H
#define COMMAND_IDS_CL_H

// Menu command IDs
enum
{
	// File Menu
	cmd_CharacterMgr	= 'ChrM',
	cmd_SelectChar		= 'SlCh',
	cmd_JoinGame		= 'LgIn',
	cmd_LeaveGame		= 'LgOf',
	cmd_ViewMovie		= 'Flik',
	cmd_RecordMovie		= 'Actn',
	
	// Edit menu
	cmd_FindText		= 'Find',
	cmd_FindAgain		= 'Fnd+',
	
	// Options menu
	cmd_Sound			= 'Snd~',
	cmd_Music			= 'Musc',
	cmd_TextStyles		= 'TxSt',
	cmd_ReloadMacros	= 'RMac',
	cmd_SaveTextLogs	= 'TLog',
	cmd_NetworkStats	= 'NtSt',
	cmd_GameWindow		= 'GAMw',
	cmd_TextWindow		= 'TXTw',
	cmd_InventoryWindow = 'INVw',
	cmd_PlayersWindow	= 'PLYw',
	
	// Command menu
	cmd_Info			= 'Info',
	cmd_Pull			= 'Pull',
	cmd_Push			= 'Push',
	cmd_Karma			= 'Krma',
	cmd_Thank			= 'Thx!',
	cmd_Curse			= '-Thx',
	cmd_Share			= 'Shar',
	cmd_Unshare			= '-Shr',
	cmd_Befriend		= 'Frnd',
	cmd_Block			= 'Blok',
	cmd_Ignore			= 'Blk+',
	cmd_Equip			= 'Eqip',
	cmd_Use				= 'Usei',
	
#ifdef DEBUG_VERSION
	// God Menu
	cmd_HideMonsterNames = 'HdMN',
	cmd_Account			= 'Acct',
	cmd_Set				= 'Set ',
	cmd_Stats			= 'Stat',
	cmd_Ranks			= 'Rank',
	cmd_Goto			= 'Goto',
	cmd_Where			= 'Wher',
	cmd_Punt			= 'Punt',
	cmd_Tricorder		= 'Tric',
	cmd_Follow			= 'Folo',
	
	// Debug Menu
	cmd_ShowFrameRate	= 'Frm#',
	cmd_SaveMsgLog		= 'MLog',
	cmd_ShowDrawTime	= 'DrwT',
	cmd_ShowImageCount	= '#Img',
	cmd_FlushCaches		= 'Loo!',	// para-scatological humor
	cmd_SetIPAddress	= 'IPv4',
	cmd_FastBlend		= '+Bln',
	cmd_SecretDrawOpts	= 'SDO!',
	cmd_DumpBlocks		= 'DmpB',
	cmd_PlayerFileStats	= 'PFSt',
	cmd_SetVersionNumber = 'StVn',
#endif  // DEBUG_VERSION
	
	// Night level menu
	cmd_NightLevel0		= 'NoNt',
	cmd_NightLevel25	= 'Nt25',
	cmd_NightLevel50	= 'Nt50',
	cmd_NightLevel75	= 'Nt75',
	cmd_NightLevel100	= 'Dark',
	
	// speech bubble opacity popup (text style dialog)
	cmd_Bubble100		= 'Bb00',
	cmd_Bubble75		= 'Bb75',
	cmd_Bubble50		= 'Bb50',
	cmd_Bubble25		= 'Bb25',
	
	// Label submenu
	cmd_LabelNone		= 'Frgt',
	cmd_LabelRed		= 'Lbl1',
	cmd_LabelOrange		= 'Lbl2',
	cmd_LabelGreen		= 'Lbl3',
	cmd_LabelBlue		= 'Lbl4',
	cmd_LabelPurple		= 'Lbl5',
	
	// Help menu
	cmd_ClientHelpURL	= 'HlpO',
};


#endif  // COMMAND_IDS_CL_H

