#!/usr/bin/env python
#
# Replacement `MtimeFileWatcher` for App Engine SDK's dev_appserver.py,
# designed for OS X. Improves upon existing file watcher (under OS X) in
# numerous ways:
#
#   - Uses FSEvents API to watch for changes instead of polling. This saves a
#     dramatic amount of CPU, especially in projects with several modules.
#   - Tries to be smarter about which modules reload when files change, only
#     modified module should reload.
#
# Install:
#   $ pip install macfsevents
#   $ cp mtime_file_watcher.py \
#        sdk/google/appengine/tools/devappserver2/mtime_file_watcher.py
import logging
import os
import threading

from google.appengine.tools.devappserver2 import watcher_common

try:
  from fsevents import Observer
  from fsevents import Stream
  from fsevents import FS_FLAGNONE
except ImportError:
  FSEvents = None
  Observer = None
  Stream = None

class FSEventsFileWatcher(object):
    SUPPORTS_MULTIPLE_DIRECTORIES = True

    def __init__(self, directories, **kwargs):
        self._directories = [os.path.abspath(d) for d in directories]
        self._watcher_ignore_re = None
        self._skip_files_re = None
        self._changes = []
        self._change_event = threading.Event()
        self.observer = Observer()

        logging.info("FSEventsFileWatcher created for %s", self._directories)

        def callback(event, mask=None):
            logging.debug("FSEventsFileWatcher event %s", event)
            if event.mask == FS_FLAGNONE:
                return

            absolute_path = event.name
            directory = next(d for d in self._directories if absolute_path.startswith(d))
            skip_files_re = self._skip_files_re
            watcher_ignore_re = self._watcher_ignore_re
            if watcher_common.ignore_file(absolute_path, skip_files_re, watcher_ignore_re):
                return

            # We also want to ignore a path if we should ignore any directory
            # that the path is in.
            def _recursive_ignore_dir(dirname):
                assert not os.path.isabs(dirname)  # or the while will never terminate
                (dir_dirpath, dir_base) = os.path.split(dirname)
                while dir_base:
                    if watcher_common.ignore_dir(dir_dirpath, dir_base, skip_files_re):
                        return True
                    if watcher_common.ignore_dir(dir_dirpath, dir_base, watcher_ignore_re):
                        return True
                    (dir_dirpath, dir_base) = os.path.split(dir_dirpath)
                return False

            relpath = os.path.relpath(absolute_path, directory)
            if _recursive_ignore_dir(os.path.dirname(relpath)):
                return

            logging.info("Reloading instances due to change in %s", absolute_path)
            self._changes.append(absolute_path)
            self._change_event.set()

        self.stream = Stream(callback, file_events=True, *self._directories)

    def set_watcher_ignore_re(self, watcher_ignore_re):
        """Allows the file watcher to ignore a custom pattern set by the user."""
        logging.debug("FSEventsFileWatcher.set_watcher_ignore_re %s", watcher_ignore_re)
        self._watcher_ignore_re = watcher_ignore_re

    def set_skip_files_re(self, skip_files_re):
        """All re's in skip_files_re are taken to be relative to its base-dir."""
        logging.debug("FSEventsFileWatcher.set_skip_files_re %s", skip_files_re)
        self._skip_files_re = skip_files_re


    def _path_ignored(self, file_path):
        """Determines if a path is ignored or not."""
        return watcher_common.ignore_file(file_path, self._skip_files_re, self._watcher_ignore_re)

    @staticmethod
    def is_available():
      return Observer is not None

    def start(self):
        self.observer.schedule(self.stream)
        self.observer.start()

    def changes(self, timeout=0):
      try:
        self._change_event.wait(timeout / 1000.0)
        changed = set(self._changes)
        return changed
      finally:
        self._changes = []
        self._change_event.clear()

    def quit(self):
        self.observer.unschedule(self.stream)
        self.observer.stop()
        self.observer.join()
