/*
**	FileRefs.r		Clanlord [all]
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
**	common file-reference mappings
*/

#include <CoreServices/CoreServices.r>
#include "DatabaseTypes_cl.h"


resource 'FREF' (rAppFREF, "Application", purgeable) {
	'APPL', 0, ""
};

resource 'FREF' (rPrefsFREF, "Prefs", purgeable) {
	'pref', 1, ""
};

resource 'FREF' (rClientImagesFREF, "Client Images", purgeable) {
	'CLi\306', 2, ""	// \306 is MacRoman "delta" (option+j)
};

resource 'FREF' (rClientSoundsFREF, "Client Sounds", purgeable) {
	'CLs\306', 3, ""
};

resource 'FREF' (rServerWorldFREF, "Server World", purgeable) {
	'CSw\306', 6, ""
};

resource 'FREF' (rServerPlayersFREF, "Server Players", purgeable) {
	'CSp\306', 7, ""
};

resource 'FREF' (rEditorUndoFREF, "Editor Undo", purgeable) {
	'CEu\306', 12, ""
};

resource 'FREF' (rEditorAreasFREF, "Editor Areas", purgeable) {
	'CEa\306', 13, ""
};

resource 'FREF' (rEditorDiffsFREF, "Editor Diffs", purgeable) {
	'CEd\306', 14, ""
};

resource 'FREF' (rClientMovieFREF, "Client Movie", purgeable) {
	'CLm\306', 15, ""
};

resource 'FREF' (rClientSignFREF, "Client Signature", purgeable) {
	kClientAppSign, 16, ""		// 'CLc\306'
};

resource 'FREF' (rServerSignFREF, "Server Signature", purgeable) {
	kServerAppSign, 17, ""		// 'CLv\306'
};

	// there is no "compiler", as such -- it's baked into the server
resource 'FREF' (rCompilerSignFREF, "Compiler Signature", purgeable) {
	kCompilerAppSign, 18, ""	// 'CLo\306'
};

resource 'FREF' (rEditorSignFREF, "Editor Signature", purgeable) {
	kEditorAppSign, 19, ""		// 'CLe\306'
};

resource 'FREF' (rClientPlayersFREF, "Client Players", purgeable) {
	'CLp\306', 20, ""
};

resource 'FREF' (rClientImagesUpdateFREF, "Client Image Update", purgeable) {
	'CL\224\306', 21, ""		// i-hat (option+i, i) delta
};

resource 'FREF' (rClientSoundsUpdateFREF, "Client Sounds Update", purgeable) {
	'CL\247\306', 22, ""		// ss (option+s) delta
};

resource 'FREF' (rEditorMarksFREF, "Editor marks", purgeable) {
	'CEmk', 23, ""
};


