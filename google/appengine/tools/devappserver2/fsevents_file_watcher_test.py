#!/usr/bin/env python
#
# Copied from mtime_file_watcher_test.py; MANY TESTS FAIL; this needs more love.
#
"""Tests for google.appengine.tools.devappserver2.fsevents_file_watcher."""

import os
import re
import shutil
import tempfile
import time
import unittest

from google.appengine.tools.devappserver2 import fsevents_file_watcher


def _sync():
  time.sleep(.1)  # just to stay over the FS timestamp resolution


class TestFSEventsFileWatcher(unittest.TestCase):
  """Tests for mtime_file_watcher.MtimeFileWatcher."""

  def setUp(self):
    self._directory = os.path.realpath(tempfile.mkdtemp())  # The watched directory
    self._junk_directory = os.path.realpath(tempfile.mkdtemp())  # A scrap directory.
    self._watcher = fsevents_file_watcher.FSEventsFileWatcher([self._directory])

  def tearDown(self):
    self._watcher.quit()
    shutil.rmtree(self._directory)
    shutil.rmtree(self._junk_directory)

  def _create_file(self, relative_path):
    realpath = os.path.realpath(os.path.join(self._directory, relative_path))
    with open(realpath, 'w'):
      pass
    return realpath

  def _create_directory(self, relative_path):
    realpath = os.path.realpath(os.path.join(self._directory, relative_path))
    os.mkdir(realpath)
    return realpath

  def test_path_ignored_with_only_skip_file_re(self):
    self._watcher.set_skip_files_re(re.compile('monkey'))
    self._watcher.start()
    self.assertTrue(self._watcher._path_ignored('monkey'))

  def test_path_ignored_with_only_watcher_ignore_re(self):
    self._watcher.set_watcher_ignore_re(re.compile('island'))
    self._watcher.start()
    self.assertTrue(self._watcher._path_ignored('island'))

  def test_path_ignored_with_both_matchers(self):
    self._watcher.set_skip_files_re(re.compile('guybrush'))
    self._watcher.set_watcher_ignore_re(re.compile('threepwood'))
    self._watcher.start()
    self.assertTrue(self._watcher._path_ignored('guybrush'))

  def test_path_not_ignored_with_both_matchers_and_no_match(self):
    self._watcher.set_skip_files_re(re.compile('revenge'))
    self._watcher.set_watcher_ignore_re(re.compile('of'))
    self._watcher.start()
    self.assertFalse(self._watcher._path_ignored('lechuck'))

  def test_file_created(self):
    self._watcher.start()
    # self._watcher._startup_thread.join()
    path = self._create_file('test')
    self.assertEqual(self._watcher.changes(100), {path})

  def test_watcher_ignore_re(self):
    self._watcher.set_watcher_ignore_re(re.compile('^.*ignored-watcher'))
    self._watcher.start()
    # self._watcher._startup_thread.join()
    self._create_file('ignored-watcher')
    self.assertEqual(self._watcher.changes(100), set())

    path = self._create_directory('subdir/')
    self.assertEqual(self._watcher.changes(100), {path})
    path = self._create_file('subdir/ignored-watcher')
    # watcher_ignore_re should also match subdirectories of watched directory.
    self.assertEqual(self._watcher.changes(100), set())

    # Avoid polluting other tests.
    self._watcher.set_watcher_ignore_re(None)

  def test_skip_file_re(self):
    self._watcher.set_skip_files_re(re.compile('^.*skipped_file'))
    self._watcher.start()
    # self._watcher._startup_thread.join()
    self._create_file('skipped_file')
    self.assertEqual(self._watcher.changes(100), set())

    path = self._create_directory('subdir/')
    self.assertEqual(self._watcher.changes(100), {path})
    path = self._create_file('subdir/skipped_file')
    # skip_files_re should also match subdirectories of watched directory.
    self.assertEqual(self._watcher.changes(100), set())

    # Avoid polluting other tests.
    self._watcher.set_skip_files_re(None)

  def test_file_modified(self):
    path = self._create_file('test')
    _sync()
    self._watcher.start()
    # self._watcher._startup_thread.join()
    with open(path, 'w') as f:
      f.write('testing')
    self.assertEqual(self._watcher.changes(100), {path})

  def test_file_read(self):
    path = self._create_file('test')
    with open(path, 'w') as f:
      f.write('testing')
    self._watcher.start()
    # self._watcher._startup_thread.join()
    with open(path, 'r') as f:
      f.read()
    # Reads should not trigger updates.
    self.assertEqual(self._watcher.changes(100), set())

  def test_file_deleted(self):
    path = self._create_file('test')
    self._watcher.start()
    # self._watcher._startup_thread.join()
    os.remove(path)
    self.assertEqual(self._watcher.changes(100), {path})

  def test_file_renamed(self):
    source = self._create_file('test')
    target = os.path.join(os.path.dirname(source), 'test2')
    self._watcher.start()
    # self._watcher._startup_thread.join()
    os.rename(source, target)
    self.assertEqual(self._watcher.changes(100), {source, target})

  def test_create_directory(self):
    self._watcher.start()
    # self._watcher._startup_thread.join()
    path = self._create_directory('test')
    self.assertEqual(self._watcher.changes(100), {path})

  def test_skip_file_re_directory(self):
    self._watcher.set_skip_files_re(re.compile('.*skipped_dir'))
    self._watcher.start()
    # self._watcher._startup_thread.join()
    self._create_directory('skipped_dir/')
    self.assertEqual(self._watcher.changes(100), set())
    # If a directory is skipped, the files and directories in that directory
    # would also be skipped
    self._create_directory('skipped_dir/subdir/')
    self.assertEqual(self._watcher.changes(100), set())

    path = self._create_directory('subdir/')
    self.assertEqual(self._watcher.changes(100), {path})
    # skip_files_re should also match subdirectories of watched directory.
    path = self._create_directory('subdir/skipped_dir/')
    self.assertEqual(self._watcher.changes(100), set())

    # Avoid polluting other tests.
    self._watcher.set_skip_files_re(None)

  def test_file_created_in_directory(self):
    self._create_directory('test')
    _sync()
    self._watcher.start()
    # self._watcher._startup_thread.join()
    path = self._create_file('test/file')
    # Keep behavior consistency with inofiy_file_watcher,
    # parent directory of path should not be considered as changed.
    self.assertEqual(self._watcher.changes(100), {path})

  def test_move_directory(self):
    source = self._create_directory('test')
    target = os.path.join(os.path.dirname(source), 'test2')
    self._watcher.start()
    # self._watcher._startup_thread.join()
    os.rename(source, target)
    self.assertEqual(self._watcher.changes(100), {source, target})

  def test_move_directory_out_of_watched(self):
    source = self._create_directory('test')
    target = os.path.join(self._junk_directory, 'test')
    self._watcher.start()
    # self._watcher._startup_thread.join()
    os.rename(source, target)
    self.assertEqual(self._watcher.changes(100), {source})
    with open(os.path.join(target, 'file'), 'w'):
      pass
    # Changes to files in subdirectories that have been moved should be ignored.
    self.assertEqual(self._watcher.changes(100), set())

  def test_move_directory_into_watched(self):
    source = os.path.join(self._junk_directory, 'source')
    target = os.path.join(self._directory, 'target')
    os.mkdir(source)
    _sync()
    self._watcher.start()
    # self._watcher._startup_thread.join()
    os.rename(source, target)
    self.assertEqual(self._watcher.changes(100), {target})
    file_path = os.path.join(target, 'file')
    with open(file_path, 'w+'):
      pass
    # Keep behavior consistency with inofiy_file_watcher,
    # target should not be considered as changed.
    self.assertEqual(self._watcher.changes(100), {file_path})

  def test_directory_deleted(self):
    path = self._create_directory('test')
    _sync()
    self._watcher.start()
    # self._watcher._startup_thread.join()
    os.rmdir(path)
    self.assertEqual(self._watcher.changes(100), {path})


if __name__ == '__main__':
  unittest.main()
