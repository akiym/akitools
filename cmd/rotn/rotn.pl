#!/usr/bin/env perl
use strict;
use warnings;
use Getopt::Long qw/:config posix_default no_ignore_case bundling permute/;

GetOptions(
    'a|all'    => \my $all,
    'f|file=s' => \my $filename,
) or die;

my $str = shift;
if (defined $filename) {
    $str = do {
        open my $fh, '<', $filename or die $!;
        local $/; <$fh>;
    };
}
$str // die "Usage: $0 [-a -f FILE] STR [SLIDES]\n";

my $slides = shift // 13;
$slides %= 26;

if ($all) {
    for my $i (1..26) {
        printf "%2d: ", $i;
        print_rotn($str, $i);
    }
} else {
    print_rotn($str, $slides);
}

sub print_rotn {
    my ($str, $slides) = @_;

    my @upper = ('A'..'Z');
    my @lower = ('a'..'z');

    if ($slides >= 0) {
        for (1 .. $slides) {
            push @upper, shift @upper;
            push @lower, shift @lower;
        }
    } else {
        for (1 .. abs $slides) {
            unshift @upper, pop @upper;
            unshift @lower, pop @lower;
        }
    }

    for my $c (split //, $str) {
        if ($c =~ /[A-Z]/) {
            print $upper[ord($c) - 0x41];
        } elsif ($c =~ /[a-z]/) {
            print $lower[ord($c) - 0x61];
        } else {
            print $c;
        }
    }
    print "\n";
}
