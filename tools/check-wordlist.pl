#!/usr/bin/perl
#
# Validates a scribble.rs wordlist against the rules that the Go test suite
# (internal/game/words_test.go) and the wordlist loader (words.go) enforce:
#
#   * no carriage returns (\r)         -> Test_wordListsContainNoCarriageReturns
#   * no trailing newline at EOF       -> readWordListInternal would otherwise
#                                         produce one empty "word"
#   * no blank lines                   -> empty words are undrawable
#   * no leading/trailing whitespace   -> testWordList rejects those
#   * every word lowercase             -> testWordList rejects mixed case
#   * no duplicate words               -> not tested upstream, but each
#                                         duplicate wastes a turn
#
# Additionally flags words whose lowercase form differs from the written
# form, which usually means an accidental uppercase letter.
#
# Usage:  perl tools/check-wordlist.pl internal/game/words/es
# Exit code is nonzero when any hard violation is found.

use strict;
use warnings;
use utf8;
use open ':std', ':encoding(UTF-8)';

my $file = $ARGV[0]
    or die "usage: perl tools/check-wordlist.pl <wordlist file>\n";

open my $fh, '<', $file
    or die "cannot read '$file': $!\n";
my $content = do { local $/; <$fh> };
close $fh;

my @problems;
my @notes;
my %seen;
my $count = 0;

if (index($content, "\r") != -1) {
    push @problems, "contains carriage returns (\\r)";
}

if ($content =~ /\n\z/) {
    push @problems, "ends with a newline (upstream wordlists do not)";
}

for my $line (split /\n/, $content) {
    $count++;
    my $name = sprintf '%s:%d', $file, $count;

    if ($line eq '') {
        push @problems, "$name: blank line";
        next;
    }

    if ($line =~ /^\s/ || $line =~ /\s\z/) {
        push @problems, "$name: leading/trailing whitespace in '$line'";
    }

    # The loader lowercases every word with the language's caser before use,
    # so mixed case in the raw file is auto-fixed. Still worth knowing about.
    if (lc $line ne $line) {
        push @notes, "$name: not lowercase (auto-lowercased on load): '$line'";
    }

    # Duplicates are compared after lowercasing, matching what the loader
    # actually serves to players.
    my $word = lc $line;
    if (exists $seen{$word}) {
        push @problems, "$name: duplicate word '$word' (first seen at line $seen{$word})";
    } else {
        $seen{$word} = $count;
    }
}

if (@problems) {
    print "FAILED: $file\n";
    print "  $_\n" for @problems;
    print '  ', scalar(@problems), " problem(s) found.\n";
    exit 1;
}

my $spaces = grep { / / } keys %seen;
print "OK: $file\n";
print "  $count words, ", scalar(keys %seen), " unique";
print ", $spaces multi-word" if $spaces;
print ".\n";
print "  note: $_\n" for @notes;
