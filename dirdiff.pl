#!/usr/bin/env perl
use strict;
use warnings;
use File::Zglob;
use Getopt::Long qw/:config posix_default no_ignore_case bundling permute/;
use Text::Diff;

# dirdiff.pl - requires: `cpanm File::Zglob Text::Diff`

GetOptions(
    'i|info' => \my $info,
) or usage();

# don't forget "quote" to prevent zsh glob expansion
my $dir1 = shift // usage();
my $dir2 = shift // usage();

# get directory name for comparison
my ($basedir1) = $dir1 =~ m!(.+)/\*\*/!;
my ($basedir2) = $dir2 =~ m!(.+)/\*\*/!;

# ignore directory
my @files1 = grep { -f $_ } zglob($dir1);
my @files2 = grep { -f $_ } zglob($dir2);

my (@diff_files, @not_found_files);

for my $file1 (@files1) {
    (my $f1 = $file1) =~ s/$basedir1//;
    my $file;
    my $found = 0;
    for my $file2 (@files2) {
        (my $f2 = $file2) =~ s/$basedir2//;
        if ($f1 eq $f2) {
            my $diff = diff($file1, $file2);
            if ($diff) {
                print $diff;
                push @diff_files, [$file1, $file2];
            }
            $found = 1;
            last;
        }
    }
    unless ($found) {
        push @not_found_files, $file1;
    }
}

if ($info) {
    print "\n=====\n";
    print 'dir1: total ' . scalar @files1 . " files\n";
    print 'dir2: total ' . scalar @files2 . " files\n";
    if (@diff_files) {
        print '    ' . $_->[0] . ' -> ' . $_->[1] . "\n" for @diff_files;
    }
    if (@not_found_files) {
        print "[?] $_\n" for @not_found_files;
    }
}

sub usage {
    die <<"USAGE";
Usage: $0 'dir1' 'dir2'
    dirdiff 'dir1/**/*.php' 'dir2/**/*.php'
    dirdiff 'falsified_sourcecode/**/*.php' 'original_sourcecode/**/*.php'
USAGE
}
