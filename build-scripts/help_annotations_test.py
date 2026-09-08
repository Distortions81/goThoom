#!/usr/bin/env python3
"""Protect the editorial rule: a highlight needs useful explanatory content."""
import json
from pathlib import Path
import sys
import unittest
from unittest.mock import patch

sys.dont_write_bytecode = True
import build_help

class AnnotationTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        data=json.loads((build_help.HELP/'images/capture.json').read_text())
        cls.screens={screen['id']:screen for screen in data['screens']}

    def test_plain_screenshot_has_no_empty_annotation_controls(self):
        html=build_help.figure(self.screens['notifications'])
        self.assertIn('<img ',html)
        self.assertNotIn('help-highlight',html)
        self.assertNotIn('help-legend',html)
        self.assertNotIn('help-toggle',html)

    def test_explanations_appear_instead_of_label_only_legends(self):
        html=build_help.figure(self.screens['add-character'])
        self.assertIn('When you later press Connect',html)
        self.assertIn('Server addresses and file locations remain shared',html)
        self.assertNotIn('<li><b>1</b> Password (optional)</li>',html)

    def test_changed_control_cannot_silently_lose_its_explanation(self):
        screen=dict(self.screens['add-character'],controls=[])
        with self.assertRaisesRegex(ValueError,'Highlight target disappeared'):
            build_help.figure(screen)

    def test_caption_alone_is_not_an_annotation(self):
        with patch.dict(build_help.ANNOTATIONS,{'notifications':[{'control':'Fallen','explanation':'Fallen'}]}):
            with self.assertRaisesRegex(ValueError,'beyond its label'):
                build_help.figure(self.screens['notifications'])

    def test_reference_omits_generic_instructions_for_obvious_controls(self):
        html=build_help.reference_section(self.screens['hotkey-editor'])
        self.assertNotIn('Select to use this action',html)
        self.assertNotIn('Enter or edit text here',html)
        self.assertNotIn('<table>',html)

if __name__=='__main__':unittest.main()
