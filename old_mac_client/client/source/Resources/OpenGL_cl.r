/*
**	OpenGL_cl.r		Clanlord
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
**		
**	resource compiler input file for the text-based resources in the client,
**	specifically the ones used by the OpenGL targets.
*/


// are we using a Nib-based "Advanced Prefs" dialog?
// (If so then many of the dialog-related resources in here aren't needed)
#define USE_HIDLOGS		1

#if ! USE_HIDLOGS
# include <Carbon/Carbon.r>
# include "TranslatableTextDefs.h"
#endif


// local IDs
enum
	{
	rOGLPrefsDlgID			= 341,		// dialog ID
	};


#if ! USE_HIDLOGS

// group box w/check for OGL items
resource 'CNTL' ( (rOGLPrefsDlgID * 100) + 17, "adv pref grp2 AM", purgeable )
{
	//{160, 23, 264, 303},
	{TXTCL_OGL_USE_OGL_COORD1, TXTCL_OGL_USE_OGL_COORD2,
		TXTCL_OGL_USE_OGL_COORD3, TXTCL_OGL_USE_OGL_COORD4},
	0,
	visible,
	1,
	0,
	kControlGroupBoxCheckBoxProc,
	0,
	TXTCL_OGL_USE_OGL	
};


resource 'DITL' ( rOGLPrefsDlgID, "Advanced PrefsAM OGL", purgeable )
{
	{
		/* [1] */
		//{391, 232, 411, 301},
		{TXTCL_OGL_OK_COORD1, TXTCL_OGL_OK_COORD2, TXTCL_OGL_OK_COORD3, TXTCL_OGL_OK_COORD4},
		Button { enabled, TXTCL_OGL_OK },
		/* [2] */
		//{391, 151, 411, 220},
		{TXTCL_OGL_CANCEL_COORD1, TXTCL_OGL_CANCEL_COORD2,
			TXTCL_OGL_CANCEL_COORD3, TXTCL_OGL_CANCEL_COORD4},
		Button { enabled, TXTCL_OGL_CANCEL },
		/* [3] */
		{25, 23, 146, 303},
		Control { enabled, 24104 },
		/* [4] */
		//{49, 34, 65, 154},
		{TXTCL_OGL_COLORS_COORD1, TXTCL_OGL_COLORS_COORD2,
			TXTCL_OGL_COLORS_COORD3, TXTCL_OGL_COLORS_COORD4},
		StaticText { disabled, TXTCL_OGL_COLORS },
		/* [5] */
		//{66, 34, 84, 154},
		{TXTCL_OGL_BYBARTYPE_COORD1, TXTCL_OGL_BYBARTYPE_COORD2,
			TXTCL_OGL_BYBARTYPE_COORD3, TXTCL_OGL_BYBARTYPE_COORD4},
		RadioButton { enabled, TXTCL_OGL_BYBARTYPE },
		/* [6] */
		//{85, 34, 103, 154},
		{TXTCL_OGL_BYBARVALUE_COORD1, TXTCL_OGL_BYBARVALUE_COORD2,
			TXTCL_OGL_BYBARVALUE_COORD3, TXTCL_OGL_BYBARVALUE_COORD4},
		RadioButton { enabled, TXTCL_OGL_BYBARVALUE },
		/* [7] */
		//{49, 170, 65, 294},
		{TXTCL_OGL_PLACEMENT_COORD1, TXTCL_OGL_PLACEMENT_COORD2,
			TXTCL_OGL_PLACEMENT_COORD3, TXTCL_OGL_PLACEMENT_COORD4},
		StaticText { disabled, TXTCL_OGL_PLACEMENT },
		/* [8] */
		//{66, 170, 84, 294},
		{TXTCL_OGL_AB_C1, TXTCL_OGL_AB_C2, TXTCL_OGL_AB_C3, TXTCL_OGL_AB_C4},
		RadioButton { enabled, TXTCL_OGL_ACROSSBOTTOM },
		/* [9] */
		//{85, 170, 103, 294},
		{TXTCL_OGL_LL_C1, TXTCL_OGL_LL_C2, TXTCL_OGL_LL_C3, TXTCL_OGL_LL_C4},
		RadioButton { enabled, TXTCL_OGL_LOWERLEFT },
		/* [10] */
		//{104, 170, 122, 294},
		{TXTCL_OGL_LR_C1, TXTCL_OGL_LR_C2, TXTCL_OGL_LR_C3, TXTCL_OGL_LR_C4},
		RadioButton { enabled, TXTCL_OGL_LOWERRIGHT },
		/* [11] */
		//{123, 170, 141, 294},
		{TXTCL_OGL_UR_C1, TXTCL_OGL_UR_C2, TXTCL_OGL_UR_C3, TXTCL_OGL_UR_C4},
		RadioButton { enabled, TXTCL_OGL_UPPERRIGHT },
		/* [12] */
		{535, 23, 551, 98},
		StaticText { disabled, "placeholder" },
		/* [13] */
		//{278, 36, 296, 286},
		{TXTCL_OGL_NT_C1, TXTCL_OGL_NT_C2, TXTCL_OGL_NT_C3, TXTCL_OGL_NT_C4},
		CheckBox { enabled, TXTCL_OGL_NOTIFYWHENFRIENDSTHINK },
		/* [14] */
		//{298, 36, 316, 286},
		{TXTCL_OGL_TS_C1, TXTCL_OGL_TS_C2, TXTCL_OGL_TS_C3, TXTCL_OGL_TS_C4},
		CheckBox { enabled, TXTCL_OGL_TREATSHARESASFRIENDS },
		/* [15] */
		//{318, 36, 336, 286},
		{TXTCL_OGL_IQ_C1, TXTCL_OGL_IQ_C2, TXTCL_OGL_IQ_C3, TXTCL_OGL_IQ_C4},
		CheckBox { enabled, TXTCL_OGL_IGNORECMDQ },
		/* [16] */
		//{338, 36, 356, 286},
		{TXTCL_OGL_LJ_C1, TXTCL_OGL_LJ_C2, TXTCL_OGL_LJ_C3, TXTCL_OGL_LJ_C4},
		CheckBox { enabled, TXTCL_OGL_STARTLOGONJOIN },
		/* [17] */
		//{358, 36, 376, 286},
		{TXTCL_OGL_NM_C1, TXTCL_OGL_NM_C2, TXTCL_OGL_NM_C3, TXTCL_OGL_NM_C4},
		CheckBox {enabled, TXTCL_OGL_NEVERLOGFORMMOVIES },
		/* [18] */
		//{160, 23, 264, 303},
		{TXTCL_OGL_BOX_C1, TXTCL_OGL_BOX_C2, TXTCL_OGL_BOX_C3, TXTCL_OGL_BOX_C4},
		Control { enabled, (rOGLPrefsDlgID * 100) + 17 },
		/* [19] */
		//{183, 34, 201, 284},
		{TXTCL_OGL_UB_C1, TXTCL_OGL_UB_C2, TXTCL_OGL_UB_C3, TXTCL_OGL_UB_C4},
		RadioButton { enabled, TXTCL_OGL_USEBESTRENDERER },
		/* [20] */
		//{202, 34, 220, 284},
		{TXTCL_OGL_IA_C1, TXTCL_OGL_IA_C2, TXTCL_OGL_IA_C3, TXTCL_OGL_IA_C4},
		RadioButton { enabled, TXTCL_OGL_IGNOREHARDWAREACC },
		/* [21] */
		//{221, 34, 239, 284},
		{TXTCL_OGL_AO_C1, TXTCL_OGL_AO_C2, TXTCL_OGL_AO_C3, TXTCL_OGL_AO_C4},
		RadioButton { enabled, TXTCL_OGL_ALLOLDACC },
		/* [22] */
		//{240, 34, 258, 284},
		{TXTCL_OGL_SE_C1, TXTCL_OGL_SE_C2, TXTCL_OGL_SE_C3, TXTCL_OGL_SE_C4},
		CheckBox { enabled, TXTCL_OGL_SUBSTEFFECTS },
	}
};


resource 'DLOG' ( rOGLPrefsDlgID, "Advanced PrefsAM OGL", purgeable )
{
	//{108, 158, 543, 484},
	{TXTCL_OGL_AP_C1, TXTCL_OGL_AP_C2, TXTCL_OGL_AP_C3, TXTCL_OGL_AP_C4},
	kWindowMovableModalDialogProc,
	invisible,
	noGoAway,
	0x0,
	rOGLPrefsDlgID,
	TXTCL_OGL_ADVPREFS,
	alertPositionMainScreen
};


resource 'dlgx' ( rOGLPrefsDlgID, purgeable )
{
	versionZero {
		kDialogFlagsUseThemeBackground
		| kDialogFlagsUseControlHierarchy
		| kDialogFlagsHandleMovableModal
		| kDialogFlagsUseThemeControls
	}
};

#endif  // USE_HIDLOGS


data 'BTbl' ( 5, purgeable )	// OpenGL color index to red map
{
	$"ffff ffff ffff ffff ffff ffff ffff ffff"
	$"ffff ffff ffff ffff ffff ffff ffff ffff"
	$"ffff ffff ffff ffff ffff ffff ffff ffff"
	$"ffff ffff ffff ffff ffff ffff ffff ffff"
	$"ffff ffff ffff ffff cccc cccc cccc cccc"
	$"cccc cccc cccc cccc cccc cccc cccc cccc"
	$"cccc cccc cccc cccc cccc cccc cccc cccc"
	$"cccc cccc cccc cccc cccc cccc cccc cccc"
	$"cccc cccc cccc cccc cccc cccc cccc cccc"
	$"9999 9999 9999 9999 9999 9999 9999 9999"
	$"9999 9999 9999 9999 9999 9999 9999 9999"
	$"9999 9999 9999 9999 9999 9999 9999 9999"
	$"9999 9999 9999 9999 9999 9999 9999 9999"
	$"9999 9999 9999 9999 6666 6666 6666 6666"
	$"6666 6666 6666 6666 6666 6666 6666 6666"
	$"6666 6666 6666 6666 6666 6666 6666 6666"
	$"6666 6666 6666 6666 6666 6666 6666 6666"
	$"6666 6666 6666 6666 6666 6666 6666 6666"
	$"3333 3333 3333 3333 3333 3333 3333 3333"
	$"3333 3333 3333 3333 3333 3333 3333 3333"
	$"3333 3333 3333 3333 3333 3333 3333 3333"
	$"3333 3333 3333 3333 3333 3333 3333 3333"
	$"3333 3333 3333 3333 0000 0000 0000 0000"
	$"0000 0000 0000 0000 0000 0000 0000 0000"
	$"0000 0000 0000 0000 0000 0000 0000 0000"
	$"0000 0000 0000 0000 0000 0000 0000 0000"
	$"0000 0000 0000 0000 0000 0000 0000 eeee"
	$"dddd bbbb aaaa 8888 7777 5555 4444 2222"
	$"1111 0000 0000 0000 0000 0000 0000 0000"
	$"0000 0000 0000 0000 0000 0000 0000 0000"
	$"0000 0000 0000 0000 0000 eeee dddd bbbb"
	$"aaaa 8888 7777 5555 4444 2222 1111 0000"
};


data 'BTbl' ( 6, purgeable )	// OpenGL color index to green map
{
	$"ffff ffff ffff ffff ffff ffff cccc cccc"
	$"cccc cccc cccc cccc 9999 9999 9999 9999"
	$"9999 9999 6666 6666 6666 6666 6666 6666"
	$"3333 3333 3333 3333 3333 3333 0000 0000"
	$"0000 0000 0000 0000 ffff ffff ffff ffff"
	$"ffff ffff cccc cccc cccc cccc cccc cccc"
	$"9999 9999 9999 9999 9999 9999 6666 6666"
	$"6666 6666 6666 6666 3333 3333 3333 3333"
	$"3333 3333 0000 0000 0000 0000 0000 0000"
	$"ffff ffff ffff ffff ffff ffff cccc cccc"
	$"cccc cccc cccc cccc 9999 9999 9999 9999"
	$"9999 9999 6666 6666 6666 6666 6666 6666"
	$"3333 3333 3333 3333 3333 3333 0000 0000"
	$"0000 0000 0000 0000 ffff ffff ffff ffff"
	$"ffff ffff cccc cccc cccc cccc cccc cccc"
	$"9999 9999 9999 9999 9999 9999 6666 6666"
	$"6666 6666 6666 6666 3333 3333 3333 3333"
	$"3333 3333 0000 0000 0000 0000 0000 0000"
	$"ffff ffff ffff ffff ffff ffff cccc cccc"
	$"cccc cccc cccc cccc 9999 9999 9999 9999"
	$"9999 9999 6666 6666 6666 6666 6666 6666"
	$"3333 3333 3333 3333 3333 3333 0000 0000"
	$"0000 0000 0000 0000 ffff ffff ffff ffff"
	$"ffff ffff cccc cccc cccc cccc cccc cccc"
	$"9999 9999 9999 9999 9999 9999 6666 6666"
	$"6666 6666 6666 6666 3333 3333 3333 3333"
	$"3333 3333 0000 0000 0000 0000 0000 0000"
	$"0000 0000 0000 0000 0000 0000 0000 0000"
	$"0000 eeee dddd bbbb aaaa 8888 7777 5555"
	$"4444 2222 1111 0000 0000 0000 0000 0000"
	$"0000 0000 0000 0000 0000 eeee dddd bbbb"
	$"aaaa 8888 7777 5555 4444 2222 1111 0000"
};


data 'BTbl' ( 7, purgeable )	// OpenGL color index to blue map
{
	$"ffff cccc 9999 6666 3333 0000 ffff cccc"
	$"9999 6666 3333 0000 ffff cccc 9999 6666"
	$"3333 0000 ffff cccc 9999 6666 3333 0000"
	$"ffff cccc 9999 6666 3333 0000 ffff cccc"
	$"9999 6666 3333 0000 ffff cccc 9999 6666"
	$"3333 0000 ffff cccc 9999 6666 3333 0000"
	$"ffff cccc 9999 6666 3333 0000 ffff cccc"
	$"9999 6666 3333 0000 ffff cccc 9999 6666"
	$"3333 0000 ffff cccc 9999 6666 3333 0000"
	$"ffff cccc 9999 6666 3333 0000 ffff cccc"
	$"9999 6666 3333 0000 ffff cccc 9999 6666"
	$"3333 0000 ffff cccc 9999 6666 3333 0000"
	$"ffff cccc 9999 6666 3333 0000 ffff cccc"
	$"9999 6666 3333 0000 ffff cccc 9999 6666"
	$"3333 0000 ffff cccc 9999 6666 3333 0000"
	$"ffff cccc 9999 6666 3333 0000 ffff cccc"
	$"9999 6666 3333 0000 ffff cccc 9999 6666"
	$"3333 0000 ffff cccc 9999 6666 3333 0000"
	$"ffff cccc 9999 6666 3333 0000 ffff cccc"
	$"9999 6666 3333 0000 ffff cccc 9999 6666"
	$"3333 0000 ffff cccc 9999 6666 3333 0000"
	$"ffff cccc 9999 6666 3333 0000 ffff cccc"
	$"9999 6666 3333 0000 ffff cccc 9999 6666"
	$"3333 0000 ffff cccc 9999 6666 3333 0000"
	$"ffff cccc 9999 6666 3333 0000 ffff cccc"
	$"9999 6666 3333 0000 ffff cccc 9999 6666"
	$"3333 0000 ffff cccc 9999 6666 3333 0000"
	$"0000 0000 0000 0000 0000 0000 0000 0000"
	$"0000 0000 0000 0000 0000 0000 0000 0000"
	$"0000 0000 0000 eeee dddd bbbb aaaa 8888"
	$"7777 5555 4444 2222 1111 eeee dddd bbbb"
	$"aaaa 8888 7777 5555 4444 2222 1111 0000"
};


data 'BTbl' ( 8, purgeable )	// rgb444 to index map
{
	$"ff f4 f3 d6 f2 f1 d5 f0 ef d4 ee ed d3 ec eb d2"
	$"ea ea f3 d6 f2 f1 d5 f0 ef d4 ee ed d3 ec eb d2"
	$"e9 e9 d0 d0 d0 cf cf cf ce ce ce cd cd cd cc cc"
	$"d1 d1 d0 d0 d0 cf cf cf ce ce ce cd cd cd cc cc"
	$"e8 e8 d0 d0 d0 cf cf cf ce ce ce cd cd cd cc cc"
	$"e7 e7 ca ca ca c9 c9 c9 c8 c8 c8 c7 c7 c7 c6 c6"
	$"cb cb ca ca ca c9 c9 c9 c8 c8 c8 c7 c7 c7 c6 c6"
	$"e6 e6 ca ca ca c9 c9 c9 c8 c8 c8 c7 c7 c7 c6 c6"
	$"e5 e5 c4 c4 c4 c3 c3 c3 c2 c2 c2 c1 c1 c1 c0 c0"
	$"c5 c5 c4 c4 c4 c3 c3 c3 c2 c2 c2 c1 c1 c1 c0 c0"
	$"e4 e4 c4 c4 c4 c3 c3 c3 c2 c2 c2 c1 c1 c1 c0 c0"
	$"e3 e3 be be be bd bd bd bc bc bc bb bb bb ba ba"
	$"bf bf be be be bd bd bd bc bc bc bb bb bb ba ba"
	$"e2 e2 be be be bd bd bd bc bc bc bb bb bb ba ba"
	$"e1 e1 b8 b8 b8 b7 b7 b7 b6 b6 b6 b5 b5 b5 b4 b4"
	$"b9 b9 b8 b8 b8 b7 b7 b7 b6 b6 b6 b5 b5 b5 b4 b4"
	$"e0 e0 f3 d6 f2 f1 d5 f0 ef d4 ee ed d3 ec eb d2"
	$"e0 fe fe d6 f2 f1 d5 f0 ef d4 ee ed d3 ec eb d2"
	$"e9 fe fd d0 d0 cf cf cf ce ce ce cd cd cd cc cc"
	$"d1 d1 d0 d0 d0 cf cf cf ce ce ce cd cd cd cc cc"
	$"e8 e8 d0 d0 d0 cf cf cf ce ce ce cd cd cd cc cc"
	$"e7 e7 ca ca ca c9 c9 c9 c8 c8 c8 c7 c7 c7 c6 c6"
	$"cb cb ca ca ca c9 c9 c9 c8 c8 c8 c7 c7 c7 c6 c6"
	$"e6 e6 ca ca ca c9 c9 c9 c8 c8 c8 c7 c7 c7 c6 c6"
	$"e5 e5 c4 c4 c4 c3 c3 c3 c2 c2 c2 c1 c1 c1 c0 c0"
	$"c5 c5 c4 c4 c4 c3 c3 c3 c2 c2 c2 c1 c1 c1 c0 c0"
	$"e4 e4 c4 c4 c4 c3 c3 c3 c2 c2 c2 c1 c1 c1 c0 c0"
	$"e3 e3 be be be bd bd bd bc bc bc bb bb bb ba ba"
	$"bf bf be be be bd bd bd bc bc bc bb bb bb ba ba"
	$"e2 e2 be be be bd bd bd bc bc bc bb bb bb ba ba"
	$"e1 e1 b8 b8 b8 b7 b7 b7 b6 b6 b6 b5 b5 b5 b4 b4"
	$"b9 b9 b8 b8 b8 b7 b7 b7 b6 b6 b6 b5 b5 b5 b4 b4"
	$"df df b2 b2 b2 b1 b1 b1 b0 b0 b0 af af af ae ae"
	$"df fe fd b2 b2 b1 b1 b1 b0 b0 b0 af af af ae ae"
	$"ad fd fd fd fd ab ab ab aa aa aa a9 a9 a9 a8 a8"
	$"ad ad fd ac ac ab ab ab aa aa aa a9 a9 a9 a8 a8"
	$"ad ad fd ac fc ab ab ab aa aa aa a9 a9 a9 a8 a8"
	$"a7 a7 a6 a6 a6 a5 a5 a5 a4 a4 a4 a3 a3 a3 a2 a2"
	$"a7 a7 a6 a6 a6 a5 a5 a5 a4 a4 a4 a3 a3 a3 a2 a2"
	$"a7 a7 a6 a6 a6 a5 a5 a5 a4 a4 a4 a3 a3 a3 a2 a2"
	$"a1 a1 a0 a0 a0 9f 9f 9f 9e 9e 9e 9d 9d 9d 9c 9c"
	$"a1 a1 a0 a0 a0 9f 9f 9f 9e 9e 9e 9d 9d 9d 9c 9c"
	$"a1 a1 a0 a0 a0 9f 9f 9f 9e 9e 9e 9d 9d 9d 9c 9c"
	$"9b 9b 9a 9a 9a 99 99 99 98 98 98 97 97 97 96 96"
	$"9b 9b 9a 9a 9a 99 99 99 98 98 98 97 97 97 96 96"
	$"9b 9b 9a 9a 9a 99 99 99 98 98 98 97 97 97 96 96"
	$"95 95 94 94 94 93 93 93 92 92 92 91 91 91 90 90"
	$"95 95 94 94 94 93 93 93 92 92 92 91 91 91 90 90"
	$"b3 b3 b2 b2 b2 b1 b1 b1 b0 b0 b0 af af af ae ae"
	$"b3 b3 b2 b2 b2 b1 b1 b1 b0 b0 b0 af af af ae ae"
	$"ad ad fd ac ac ab ab ab aa aa aa a9 a9 a9 a8 a8"
	$"ad ad ac ac ac ab ab ab aa aa aa a9 a9 a9 a8 a8"
	$"ad ad ac ac fc ab ab ab aa aa aa a9 a9 a9 a8 a8"
	$"a7 a7 a6 a6 a6 a5 a5 a5 a4 a4 a4 a3 a3 a3 a2 a2"
	$"a7 a7 a6 a6 a6 a5 a5 a5 a4 a4 a4 a3 a3 a3 a2 a2"
	$"a7 a7 a6 a6 a6 a5 a5 a5 a4 a4 a4 a3 a3 a3 a2 a2"
	$"a1 a1 a0 a0 a0 9f 9f 9f 9e 9e 9e 9d 9d 9d 9c 9c"
	$"a1 a1 a0 a0 a0 9f 9f 9f 9e 9e 9e 9d 9d 9d 9c 9c"
	$"a1 a1 a0 a0 a0 9f 9f 9f 9e 9e 9e 9d 9d 9d 9c 9c"
	$"9b 9b 9a 9a 9a 99 99 99 98 98 98 97 97 97 96 96"
	$"9b 9b 9a 9a 9a 99 99 99 98 98 98 97 97 97 96 96"
	$"9b 9b 9a 9a 9a 99 99 99 98 98 98 97 97 97 96 96"
	$"95 95 94 94 94 93 93 93 92 92 92 91 91 91 90 90"
	$"95 95 94 94 94 93 93 93 92 92 92 91 91 91 90 90"
	$"de de b2 b2 b2 b1 b1 b1 b0 b0 b0 af af af ae ae"
	$"de de b2 b2 b2 b1 b1 b1 b0 b0 b0 af af af ae ae"
	$"ad ad fd ac fc ab ab ab aa aa aa a9 a9 a9 a8 a8"
	$"ad ad ac ac fc ab ab ab aa aa aa a9 a9 a9 a8 a8"
	$"ad ad fc fc fc fc ab ab aa aa aa a9 a9 a9 a8 a8"
	$"a7 a7 a6 a6 fc fb a5 a5 a4 a4 a4 a3 a3 a3 a2 a2"
	$"a7 a7 a6 a6 a6 a5 a5 a5 a4 a4 a4 a3 a3 a3 a2 a2"
	$"a7 a7 a6 a6 a6 a5 a5 a5 a4 a4 a4 a3 a3 a3 a2 a2"
	$"a1 a1 a0 a0 a0 9f 9f 9f 9e 9e 9e 9d 9d 9d 9c 9c"
	$"a1 a1 a0 a0 a0 9f 9f 9f 9e 9e 9e 9d 9d 9d 9c 9c"
	$"a1 a1 a0 a0 a0 9f 9f 9f 9e 9e 9e 9d 9d 9d 9c 9c"
	$"9b 9b 9a 9a 9a 99 99 99 98 98 98 97 97 97 96 96"
	$"9b 9b 9a 9a 9a 99 99 99 98 98 98 97 97 97 96 96"
	$"9b 9b 9a 9a 9a 99 99 99 98 98 98 97 97 97 96 96"
	$"95 95 94 94 94 93 93 93 92 92 92 91 91 91 90 90"
	$"95 95 94 94 94 93 93 93 92 92 92 91 91 91 90 90"
	$"dd dd 8e 8e 8e 8d 8d 8d 8c 8c 8c 8b 8b 8b 8a 8a"
	$"dd dd 8e 8e 8e 8d 8d 8d 8c 8c 8c 8b 8b 8b 8a 8a"
	$"89 89 88 88 88 87 87 87 86 86 86 85 85 85 84 84"
	$"89 89 88 88 88 87 87 87 86 86 86 85 85 85 84 84"
	$"89 89 88 88 fc fb 87 87 86 86 86 85 85 85 84 84"
	$"83 83 82 82 fb fb fb fb 80 80 80 7f 7f 7f 7e 7e"
	$"83 83 82 82 82 fb 81 81 80 80 80 7f 7f 7f 7e 7e"
	$"83 83 82 82 82 fb 81 fa 80 80 80 7f 7f 7f 7e 7e"
	$"7d 7d 7c 7c 7c 7b 7b 7b 7a 7a 7a 79 79 79 78 78"
	$"7d 7d 7c 7c 7c 7b 7b 7b 7a 7a 7a 79 79 79 78 78"
	$"7d 7d 7c 7c 7c 7b 7b 7b 7a 7a 7a 79 79 79 78 78"
	$"77 77 76 76 76 75 75 75 74 74 74 73 73 73 72 72"
	$"77 77 76 76 76 75 75 75 74 74 74 73 73 73 72 72"
	$"77 77 76 76 76 75 75 75 74 74 74 73 73 73 72 72"
	$"71 71 70 70 70 6f 6f 6f 6e 6e 6e 6d 6d 6d 6c 6c"
	$"71 71 70 70 70 6f 6f 6f 6e 6e 6e 6d 6d 6d 6c 6c"
	$"8f 8f 8e 8e 8e 8d 8d 8d 8c 8c 8c 8b 8b 8b 8a 8a"
	$"8f 8f 8e 8e 8e 8d 8d 8d 8c 8c 8c 8b 8b 8b 8a 8a"
	$"89 89 88 88 88 87 87 87 86 86 86 85 85 85 84 84"
	$"89 89 88 88 88 87 87 87 86 86 86 85 85 85 84 84"
	$"89 89 88 88 88 87 87 87 86 86 86 85 85 85 84 84"
	$"83 83 82 82 82 fb 81 81 80 80 80 7f 7f 7f 7e 7e"
	$"83 83 82 82 82 81 81 81 80 80 80 7f 7f 7f 7e 7e"
	$"83 83 82 82 82 81 81 fa 80 80 80 7f 7f 7f 7e 7e"
	$"7d 7d 7c 7c 7c 7b 7b 7b 7a 7a 7a 79 79 79 78 78"
	$"7d 7d 7c 7c 7c 7b 7b 7b 7a 7a 7a 79 79 79 78 78"
	$"7d 7d 7c 7c 7c 7b 7b 7b 7a 7a 7a 79 79 79 78 78"
	$"77 77 76 76 76 75 75 75 74 74 74 73 73 73 72 72"
	$"77 77 76 76 76 75 75 75 74 74 74 73 73 73 72 72"
	$"77 77 76 76 76 75 75 75 74 74 74 73 73 73 72 72"
	$"71 71 70 70 70 6f 6f 6f 6e 6e 6e 6d 6d 6d 6c 6c"
	$"71 71 70 70 70 6f 6f 6f 6e 6e 6e 6d 6d 6d 6c 6c"
	$"dc dc 8e 8e 8e 8d 8d 8d 8c 8c 8c 8b 8b 8b 8a 8a"
	$"dc dc 8e 8e 8e 8d 8d 8d 8c 8c 8c 8b 8b 8b 8a 8a"
	$"89 89 88 88 88 87 87 87 86 86 86 85 85 85 84 84"
	$"89 89 88 88 88 87 87 87 86 86 86 85 85 85 84 84"
	$"89 89 88 88 88 87 87 87 86 86 86 85 85 85 84 84"
	$"83 83 82 82 82 fb 81 fa 80 80 80 7f 7f 7f 7e 7e"
	$"83 83 82 82 82 81 81 fa 80 80 80 7f 7f 7f 7e 7e"
	$"83 83 82 82 82 fa fa fa fa 80 80 7f 7f 7f 7e 7e"
	$"7d 7d 7c 7c 7c 7b 7b fa f9 7a 7a 79 79 79 78 78"
	$"7d 7d 7c 7c 7c 7b 7b 7b 7a 7a 7a 79 79 79 78 78"
	$"7d 7d 7c 7c 7c 7b 7b 7b 7a 7a 7a 79 79 79 78 78"
	$"77 77 76 76 76 75 75 75 74 74 74 73 73 73 72 72"
	$"77 77 76 76 76 75 75 75 74 74 74 73 73 73 72 72"
	$"77 77 76 76 76 75 75 75 74 74 74 73 73 73 72 72"
	$"71 71 70 70 70 6f 6f 6f 6e 6e 6e 6d 6d 6d 6c 6c"
	$"71 71 70 70 70 6f 6f 6f 6e 6e 6e 6d 6d 6d 6c 6c"
	$"db db 6a 6a 6a 69 69 69 68 68 68 67 67 67 66 66"
	$"db db 6a 6a 6a 69 69 69 68 68 68 67 67 67 66 66"
	$"65 65 64 64 64 63 63 63 62 62 62 61 61 61 60 60"
	$"65 65 64 64 64 63 63 63 62 62 62 61 61 61 60 60"
	$"65 65 64 64 64 63 63 63 62 62 62 61 61 61 60 60"
	$"5f 5f 5e 5e 5e 5d 5d 5d 5c 5c 5c 5b 5b 5b 5a 5a"
	$"5f 5f 5e 5e 5e 5d 5d 5d 5c 5c 5c 5b 5b 5b 5a 5a"
	$"5f 5f 5e 5e 5e 5d 5d fa f9 5c 5c 5b 5b 5b 5a 5a"
	$"59 59 58 58 58 57 57 f9 f9 f9 f9 55 55 55 54 54"
	$"59 59 58 58 58 57 57 57 f9 56 56 55 55 55 54 54"
	$"59 59 58 58 58 57 57 57 f9 56 f8 55 55 55 54 54"
	$"53 53 52 52 52 51 51 51 50 50 50 4f 4f 4f 4e 4e"
	$"53 53 52 52 52 51 51 51 50 50 50 4f 4f 4f 4e 4e"
	$"53 53 52 52 52 51 51 51 50 50 50 4f 4f 4f 4e 4e"
	$"4d 4d 4c 4c 4c 4b 4b 4b 4a 4a 4a 49 49 49 48 48"
	$"4d 4d 4c 4c 4c 4b 4b 4b 4a 4a 4a 49 49 49 48 48"
	$"6b 6b 6a 6a 6a 69 69 69 68 68 68 67 67 67 66 66"
	$"6b 6b 6a 6a 6a 69 69 69 68 68 68 67 67 67 66 66"
	$"65 65 64 64 64 63 63 63 62 62 62 61 61 61 60 60"
	$"65 65 64 64 64 63 63 63 62 62 62 61 61 61 60 60"
	$"65 65 64 64 64 63 63 63 62 62 62 61 61 61 60 60"
	$"5f 5f 5e 5e 5e 5d 5d 5d 5c 5c 5c 5b 5b 5b 5a 5a"
	$"5f 5f 5e 5e 5e 5d 5d 5d 5c 5c 5c 5b 5b 5b 5a 5a"
	$"5f 5f 5e 5e 5e 5d 5d 5d 5c 5c 5c 5b 5b 5b 5a 5a"
	$"59 59 58 58 58 57 57 57 f9 56 56 55 55 55 54 54"
	$"59 59 58 58 58 57 57 57 56 56 56 55 55 55 54 54"
	$"59 59 58 58 58 57 57 57 56 56 f8 55 55 55 54 54"
	$"53 53 52 52 52 51 51 51 50 50 50 4f 4f 4f 4e 4e"
	$"53 53 52 52 52 51 51 51 50 50 50 4f 4f 4f 4e 4e"
	$"53 53 52 52 52 51 51 51 50 50 50 4f 4f 4f 4e 4e"
	$"4d 4d 4c 4c 4c 4b 4b 4b 4a 4a 4a 49 49 49 48 48"
	$"4d 4d 4c 4c 4c 4b 4b 4b 4a 4a 4a 49 49 49 48 48"
	$"da da 6a 6a 6a 69 69 69 68 68 68 67 67 67 66 66"
	$"da da 6a 6a 6a 69 69 69 68 68 68 67 67 67 66 66"
	$"65 65 64 64 64 63 63 63 62 62 62 61 61 61 60 60"
	$"65 65 64 64 64 63 63 63 62 62 62 61 61 61 60 60"
	$"65 65 64 64 64 63 63 63 62 62 62 61 61 61 60 60"
	$"5f 5f 5e 5e 5e 5d 5d 5d 5c 5c 5c 5b 5b 5b 5a 5a"
	$"5f 5f 5e 5e 5e 5d 5d 5d 5c 5c 5c 5b 5b 5b 5a 5a"
	$"5f 5f 5e 5e 5e 5d 5d 5d 5c 5c 5c 5b 5b 5b 5a 5a"
	$"59 59 58 58 58 57 57 57 f9 56 f8 55 55 55 54 54"
	$"59 59 58 58 58 57 57 57 56 56 f8 55 55 55 54 54"
	$"59 59 58 58 58 57 57 57 f8 f8 f8 f8 55 55 54 54"
	$"53 53 52 52 52 51 51 51 50 50 f8 f7 4f 4f 4e 4e"
	$"53 53 52 52 52 51 51 51 50 50 50 4f 4f 4f 4e 4e"
	$"53 53 52 52 52 51 51 51 50 50 50 4f 4f 4f 4e 4e"
	$"4d 4d 4c 4c 4c 4b 4b 4b 4a 4a 4a 49 49 49 48 48"
	$"4d 4d 4c 4c 4c 4b 4b 4b 4a 4a 4a 49 49 49 48 48"
	$"d9 d9 46 46 46 45 45 45 44 44 44 43 43 43 42 42"
	$"d9 d9 46 46 46 45 45 45 44 44 44 43 43 43 42 42"
	$"41 41 40 40 40 3f 3f 3f 3e 3e 3e 3d 3d 3d 3c 3c"
	$"41 41 40 40 40 3f 3f 3f 3e 3e 3e 3d 3d 3d 3c 3c"
	$"41 41 40 40 40 3f 3f 3f 3e 3e 3e 3d 3d 3d 3c 3c"
	$"3b 3b 3a 3a 3a 39 39 39 38 38 38 37 37 37 36 36"
	$"3b 3b 3a 3a 3a 39 39 39 38 38 38 37 37 37 36 36"
	$"3b 3b 3a 3a 3a 39 39 39 38 38 38 37 37 37 36 36"
	$"35 35 34 34 34 33 33 33 32 32 32 31 31 31 30 30"
	$"35 35 34 34 34 33 33 33 32 32 32 31 31 31 30 30"
	$"35 35 34 34 34 33 33 33 32 32 f8 f7 31 31 30 30"
	$"2f 2f 2e 2e 2e 2d 2d 2d 2c 2c f7 f7 f7 f7 2a 2a"
	$"2f 2f 2e 2e 2e 2d 2d 2d 2c 2c 2c f7 2b 2b 2a 2a"
	$"2f 2f 2e 2e 2e 2d 2d 2d 2c 2c 2c f7 2b f6 2a 2a"
	$"29 29 28 28 28 27 27 27 26 26 26 25 25 25 24 24"
	$"29 29 28 28 28 27 27 27 26 26 26 25 25 25 24 24"
	$"47 47 46 46 46 45 45 45 44 44 44 43 43 43 42 42"
	$"47 47 46 46 46 45 45 45 44 44 44 43 43 43 42 42"
	$"41 41 40 40 40 3f 3f 3f 3e 3e 3e 3d 3d 3d 3c 3c"
	$"41 41 40 40 40 3f 3f 3f 3e 3e 3e 3d 3d 3d 3c 3c"
	$"41 41 40 40 40 3f 3f 3f 3e 3e 3e 3d 3d 3d 3c 3c"
	$"3b 3b 3a 3a 3a 39 39 39 38 38 38 37 37 37 36 36"
	$"3b 3b 3a 3a 3a 39 39 39 38 38 38 37 37 37 36 36"
	$"3b 3b 3a 3a 3a 39 39 39 38 38 38 37 37 37 36 36"
	$"35 35 34 34 34 33 33 33 32 32 32 31 31 31 30 30"
	$"35 35 34 34 34 33 33 33 32 32 32 31 31 31 30 30"
	$"35 35 34 34 34 33 33 33 32 32 32 31 31 31 30 30"
	$"2f 2f 2e 2e 2e 2d 2d 2d 2c 2c 2c f7 2b 2b 2a 2a"
	$"2f 2f 2e 2e 2e 2d 2d 2d 2c 2c 2c 2b 2b 2b 2a 2a"
	$"2f 2f 2e 2e 2e 2d 2d 2d 2c 2c 2c 2b 2b f6 2a 2a"
	$"29 29 28 28 28 27 27 27 26 26 26 25 25 25 24 24"
	$"29 29 28 28 28 27 27 27 26 26 26 25 25 25 24 24"
	$"d8 d8 46 46 46 45 45 45 44 44 44 43 43 43 42 42"
	$"d8 d8 46 46 46 45 45 45 44 44 44 43 43 43 42 42"
	$"41 41 40 40 40 3f 3f 3f 3e 3e 3e 3d 3d 3d 3c 3c"
	$"41 41 40 40 40 3f 3f 3f 3e 3e 3e 3d 3d 3d 3c 3c"
	$"41 41 40 40 40 3f 3f 3f 3e 3e 3e 3d 3d 3d 3c 3c"
	$"3b 3b 3a 3a 3a 39 39 39 38 38 38 37 37 37 36 36"
	$"3b 3b 3a 3a 3a 39 39 39 38 38 38 37 37 37 36 36"
	$"3b 3b 3a 3a 3a 39 39 39 38 38 38 37 37 37 36 36"
	$"35 35 34 34 34 33 33 33 32 32 32 31 31 31 30 30"
	$"35 35 34 34 34 33 33 33 32 32 32 31 31 31 30 30"
	$"35 35 34 34 34 33 33 33 32 32 32 31 31 31 30 30"
	$"2f 2f 2e 2e 2e 2d 2d 2d 2c 2c 2c f7 2b f6 2a 2a"
	$"2f 2f 2e 2e 2e 2d 2d 2d 2c 2c 2c 2b 2b f6 2a 2a"
	$"2f 2f 2e 2e 2e 2d 2d 2d 2c 2c 2c f6 f6 f6 f6 2a"
	$"29 29 28 28 28 27 27 27 26 26 26 25 25 f6 f5 24"
	$"29 29 28 28 28 27 27 27 26 26 26 25 25 25 24 24"
	$"d7 d7 22 22 22 21 21 21 20 20 20 1f 1f 1f 1e 1e"
	$"d7 d7 22 22 22 21 21 21 20 20 20 1f 1f 1f 1e 1e"
	$"1d 1d 1c 1c 1c 1b 1b 1b 1a 1a 1a 19 19 19 18 18"
	$"1d 1d 1c 1c 1c 1b 1b 1b 1a 1a 1a 19 19 19 18 18"
	$"1d 1d 1c 1c 1c 1b 1b 1b 1a 1a 1a 19 19 19 18 18"
	$"17 17 16 16 16 15 15 15 14 14 14 13 13 13 12 12"
	$"17 17 16 16 16 15 15 15 14 14 14 13 13 13 12 12"
	$"17 17 16 16 16 15 15 15 14 14 14 13 13 13 12 12"
	$"11 11 10 10 10 0f 0f 0f 0e 0e 0e 0d 0d 0d 0c 0c"
	$"11 11 10 10 10 0f 0f 0f 0e 0e 0e 0d 0d 0d 0c 0c"
	$"11 11 10 10 10 0f 0f 0f 0e 0e 0e 0d 0d 0d 0c 0c"
	$"0b 0b 0a 0a 0a 09 09 09 08 08 08 07 07 07 06 06"
	$"0b 0b 0a 0a 0a 09 09 09 08 08 08 07 07 07 06 06"
	$"0b 0b 0a 0a 0a 09 09 09 08 08 08 07 07 f6 f5 06"
	$"05 05 04 04 04 03 03 03 02 02 02 01 01 f5 f5 f5"
	$"05 05 04 04 04 03 03 03 02 02 02 01 01 01 f5 00"
	$"23 23 22 22 22 21 21 21 20 20 20 1f 1f 1f 1e 1e"
	$"23 23 22 22 22 21 21 21 20 20 20 1f 1f 1f 1e 1e"
	$"1d 1d 1c 1c 1c 1b 1b 1b 1a 1a 1a 19 19 19 18 18"
	$"1d 1d 1c 1c 1c 1b 1b 1b 1a 1a 1a 19 19 19 18 18"
	$"1d 1d 1c 1c 1c 1b 1b 1b 1a 1a 1a 19 19 19 18 18"
	$"17 17 16 16 16 15 15 15 14 14 14 13 13 13 12 12"
	$"17 17 16 16 16 15 15 15 14 14 14 13 13 13 12 12"
	$"17 17 16 16 16 15 15 15 14 14 14 13 13 13 12 12"
	$"11 11 10 10 10 0f 0f 0f 0e 0e 0e 0d 0d 0d 0c 0c"
	$"11 11 10 10 10 0f 0f 0f 0e 0e 0e 0d 0d 0d 0c 0c"
	$"11 11 10 10 10 0f 0f 0f 0e 0e 0e 0d 0d 0d 0c 0c"
	$"0b 0b 0a 0a 0a 09 09 09 08 08 08 07 07 07 06 06"
	$"0b 0b 0a 0a 0a 09 09 09 08 08 08 07 07 07 06 06"
	$"0b 0b 0a 0a 0a 09 09 09 08 08 08 07 07 07 06 06"
	$"05 05 04 04 04 03 03 03 02 02 02 01 01 01 f5 00"
	$"05 05 04 04 04 03 03 03 02 02 02 01 01 01 00 00"
};

